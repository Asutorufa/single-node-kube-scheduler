package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type Kubernetes struct {
	node   atomic.Pointer[string]
	Client *kubernetes.Clientset
}

func newKubernetes() (*Kubernetes, error) {
	var apiConfig *clientcmdapi.Config
	var err error
	apiConfig, _ = clientcmd.NewDefaultClientConfigLoadingRules().Load()
	var config *rest.Config
	if apiConfig != nil {
		config, err = clientcmd.NewDefaultClientConfig(*apiConfig, nil).ClientConfig()
	}
	if config == nil || err != nil {
		fmt.Println(err)
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}

	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	kcli := &Kubernetes{
		Client: cli,
	}

	node, err := kcli.GetNodes(context.TODO())
	if err != nil {
		return nil, err
	}

	kcli.node.Store(&node)

	return kcli, nil
}

func (k *Kubernetes) GetNodes(ctx context.Context) (string, error) {
	nodes, err := k.Client.CoreV1().Nodes().List(ctx, v1.ListOptions{})
	if err != nil {
		return "", err
	}

	if len(nodes.Items) != 1 {
		return "", fmt.Errorf("single node kube scheduler only support one node")
	}

	return nodes.Items[0].Name, nil
}

type Pod struct {
	Name      string
	Uid       types.UID
	Namepsace string
	Image     []string
}

func (k *Kubernetes) setAlreadyExists(ctx context.Context) error {
	ls, err := k.Client.CoreV1().Pods("").List(ctx, v1.ListOptions{
		FieldSelector: "status.phase=Pending,spec.nodeName=",
	})
	if err != nil {
		return err
	}

	slog.Info("set already exists", "count", len(ls.Items))

	for _, l := range ls.Items {
		if l.Spec.NodeName != "" {
			slog.Info("pod already have node, skip", "namespace", l.Namespace, "name", l.Name, "node", l.Spec.NodeName)
			continue
		}
		binding := &corev1.Binding{
			ObjectMeta: metav1.ObjectMeta{Namespace: l.Namespace, Name: l.Name, UID: l.UID},
			Target:     corev1.ObjectReference{Kind: "Node", Name: *k.node.Load()},
		}

		err := k.Client.CoreV1().Pods(l.Namespace).Bind(
			ctx,
			binding,
			v1.CreateOptions{},
		)
		if err != nil {
			slog.Error("patch pod error", "namespace", l.Namespace, "name", l.Name, "error", err)
		}
	}

	return nil
}

func (k *Kubernetes) Run(ctx context.Context) {
	go k.startWatchNode(ctx)

	if err := k.setAlreadyExists(ctx); err != nil {
		slog.Error("set already exists", "error", err)

		go func() {
			slog.Info("run set already exists un")
			for {
				time.Sleep(5 * time.Second)
				if err := k.setAlreadyExists(ctx); err != nil {
					slog.Error("set already exists", "error", err)
					continue
				}

				break
			}
		}()
	}

	for {
		err := k.startWatch(ctx)
		if err != nil {
			slog.Error("start watch", "error", err)
		}

		time.Sleep(5 * time.Second)
	}
}

func (k *Kubernetes) startWatch(ctx context.Context) error {
	watcher, err := k.Client.CoreV1().Pods("").Watch(ctx, v1.ListOptions{
		Watch: true,
	})
	if err != nil {
		return err
	}

	for event := range watcher.ResultChan() {
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}

		slog.Info("pod event", "namespace", pod.Namespace, "name", pod.Name, "node_name", pod.Spec.NodeName, "type", event.Type)

		switch event.Type {
		case watch.Added, watch.Modified:
			if pod.Spec.NodeName != "" {
				continue
			}

			slog.Info("set pod node", "namespace", pod.Namespace, "name", pod.Name, "node", *k.node.Load())

			binding := &corev1.Binding{
				ObjectMeta: metav1.ObjectMeta{Namespace: pod.Namespace, Name: pod.Name, UID: pod.UID},
				Target:     corev1.ObjectReference{Kind: "Node", Name: *k.node.Load()},
			}

			err := k.Client.CoreV1().Pods(pod.Namespace).Bind(
				ctx,
				binding,
				v1.CreateOptions{},
			)
			if err != nil {
				slog.Error("patch pod error", "namespace", pod.Namespace, "name", pod.Name, "error", err)
			}
		}
	}

	return nil
}

func (k *Kubernetes) startWatchNode(ctx context.Context) {
	slog.Info("start watch node")

	for {
		watcher, err := k.Client.CoreV1().Nodes().Watch(ctx, v1.ListOptions{
			Watch: true,
		})
		if err != nil {
			slog.Error("start watch node", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for event := range watcher.ResultChan() {
			node, ok := event.Object.(*corev1.Node)
			if !ok {
				continue
			}

			args := []any{
				slog.String("node", node.Name),
				slog.Any("type", event.Type),
			}

			for _, v := range node.Status.Conditions {
				args = append(args, slog.String(string(v.Type), string(v.Status)))
			}

			slog.Info("check node changed, relist pods", args...)

			if (event.Type == watch.Added || event.Type == watch.Modified) && node.Name != "" {
				k.node.Store(&node.Name)
			}

			for {
				time.Sleep(5 * time.Second)
				if err := k.setAlreadyExists(ctx); err != nil {
					slog.Error("set already exists", "error", err)
					continue
				}

				break
			}
		}

		time.Sleep(5 * time.Second)
	}
}

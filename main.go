package main

import "context"

func main() {
	k, err := newKubernetes()
	if err != nil {
		panic(err)
	}

	k.Run(context.TODO())
}

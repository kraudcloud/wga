package main

import (
	"github.com/kraudcloud/wga/apis/wga.kraudcloud.com/v1beta"
	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
)

func main() {
	controllergen.Run(args.Options{
		OutputPackage: "github.com/kraudcloud/wga/apis/generated",
		// required by controllergen
		Boilerplate: "codegen/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"wga.kraudcloud.com": {
				Types: []interface{}{
					v1beta.WireguardAccessPeer{},
					v1beta.WireguardAccessRule{},
					v1beta.WireguardClusterClient{},
				},
				GenerateTypes:     true,
				GenerateClients:   true,
				GenerateListers:   true,
				GenerateInformers: true,
			},
		},
	})
}

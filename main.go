package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/langfuse/terraform-provider-langfuse/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/cresta/langfuse",
		Debug:   debug,
	}

	providerVersion := fmt.Sprintf("terraform-provider-langfuse: version %s (Commit: %s Date: %s)\n", version, commit, date)

	err := providerserver.Serve(context.Background(), provider.New(providerVersion), opts)

	if err != nil {
		log.Fatal(err.Error())
	}
}

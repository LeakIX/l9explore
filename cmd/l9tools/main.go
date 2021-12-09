package main

import (
	"github.com/LeakIX/l9explore"
	"github.com/alecthomas/kong"
)

var App struct {
	ExploreService l9explore.ExploreServiceCommand `cmd name:"service" help:"Explores services"`

}

func main() {
	ctx := kong.Parse(&App)
	// Call the Run() method of the selected parsed command.
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
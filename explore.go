package l9explore

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"github.com/LeakIX/l9format"
	"github.com/gboddin/goccm"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"time"
)

type ExploreServiceCommand struct {
	PluginDir      string                            `type:"existingdir" short:"s" default:"~/.l9/plugins/service"`
	MaxThreads     int                               `help:"Max threads" short:"t" default:"10"`
	OnlyLeak       bool                              `help:"Discards services events" short:"l"`
	OpenPlugins    []l9format.ServicePluginInterface `kong:"-"`
	ExplorePlugins []l9format.ServicePluginInterface `kong:"-"`
	ThreadManager  *goccm.ConcurrencyManager         `kong:"-"`
	JsonEncoder    *json.Encoder                     `kong:"-"`
	ExploreTimeout time.Duration                     `short:"x" default:"3s"`
	Debug          bool
}

func (cmd *ExploreServiceCommand) Run() error {
	if !cmd.Debug {
		log.SetOutput(ioutil.Discard)
	}
	stdinReader := bufio.NewReaderSize(os.Stdin, 256*1024)
	cmd.JsonEncoder = json.NewEncoder(os.Stdout)
	err := cmd.LoadPlugins()
	if err != nil {
		return err
	}
	cmd.ThreadManager = goccm.New(cmd.MaxThreads)
	defer cmd.ThreadManager.WaitAllDone()
	for {
		bytes, isPrefix, err := stdinReader.ReadLine()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Fatal(err)
		}
		if isPrefix == true {
			log.Fatal("Event is too big")
		}
		event := &l9format.L9Event{}
		err = json.Unmarshal(bytes, event)
		if err != nil {
			return err
		}
		cmd.ThreadManager.Wait()
		event.AddSource("l9explore")
		go cmd.Explore(event)
	}
	return nil
}

func (cmd *ExploreServiceCommand) Explore(event *l9format.L9Event) {
	// send to open plugins
	ctx, cancel := context.WithTimeout(context.Background(), cmd.ExploreTimeout)
	defer cancel()
	defer cmd.ThreadManager.Done()
	serviceSummary := event.Summary
	for _, loadedPlugin := range cmd.OpenPlugins {
		if event.MatchServicePlugin(loadedPlugin) {
			if leak, hasLeak := loadedPlugin.Run(ctx, event); hasLeak {
				event.Leak = leak
				event.EventType = "leak"
				event.Summary += "\n" + leak.Data
				event.AddSource(loadedPlugin.GetName())
				cmd.JsonEncoder.Encode(event)
			}
		}
	}
	if !cmd.OnlyLeak {
		event.Summary = serviceSummary
		event.EventType = "service"
		cmd.JsonEncoder.Encode(event)
	}
}
func (cmd *ExploreServiceCommand) LoadPlugins() error {
	pluginsToLoad, _ := filepath.Glob(cmd.PluginDir + "/*.so")
	for _, pluginToLoad := range pluginsToLoad {
		p, err := plugin.Open(pluginToLoad)
		if err != nil {
			return err
		}
		symbol, _ := p.Lookup("New")
		pluginFactory, ok := symbol.(func() l9format.ServicePluginInterface)
		if !ok {
			return errors.New("plugins does not implement New")
		}
		if pluginFactory().GetStage() == "open" {
			cmd.OpenPlugins = append(cmd.OpenPlugins, pluginFactory())
		} else {
			cmd.ExplorePlugins = append(cmd.ExplorePlugins, pluginFactory())
		}
		majorVersion, minorVersion, patchVersion := pluginFactory().GetVersion()
		log.Printf("Plugin %s %d.%d.%d loaded for protocols %s. Stage: %s",
			pluginFactory().GetName(), majorVersion, minorVersion, patchVersion, strings.Join(pluginFactory().GetProtocols(), ", "), pluginFactory().GetStage())
	}
	return nil
}

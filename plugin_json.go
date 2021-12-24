package l9explore

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/LeakIX/l9format"
)

// GetServicePluginFromString returns the service plugin with the given name
func GetServicePluginFromString(pluginName string) (plugin l9format.ServicePluginInterface, found bool) {
	for _, tcpPlugin := range tcpPlugins {
		if tcpPlugin.GetName() == pluginName {
			return tcpPlugin, true
		}
	}
	return nil, false
}

// GetWebPluginFromString returns the web plugin with the given name
func GetWebPluginFromString(pluginName string) (plugin l9format.WebPluginInterface, found bool) {
	for _, webPlugin := range webPlugins {
		if webPlugin.GetName() == pluginName {
			return webPlugin, true
		}
	}
	return nil, false
}

const PluginFile = "plugins.json"

type PluginFileStruct struct {
	Plugins []string `json:"plugins"`
}

func LoadPluginsFromFile(pluginsJson string) error {

	jsonFile, err := os.Open(pluginsJson)
	if err != nil {
		return err
	}

	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	// we initialize our Users array
	var fileData PluginFileStruct

	err = json.Unmarshal(byteValue, &fileData)
	if err != nil {
		return err
	}

	TcpPlugins = []l9format.ServicePluginInterface{}
	WebPlugins = []l9format.WebPluginInterface{}

	for _, pluginName := range fileData.Plugins {
		splug, found := GetServicePluginFromString(pluginName)
		if found {
			TcpPlugins = append(TcpPlugins, splug)
			continue
		}

		wplug, found := GetWebPluginFromString(pluginName)
		if found {
			WebPlugins = append(WebPlugins, wplug)
			continue
		}

	}

	return nil
}

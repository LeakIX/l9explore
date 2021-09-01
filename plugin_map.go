package l9explore

import (
	"github.com/LeakIX/l9format"
	// Import your plugins here"
	"github.com/LeakIX/l9plugins"
	l9_nuclei_plugin "github.com/gboddin/l9-nuclei-plugin"
)

var tcpPlugins = l9plugins.GetTcpPlugins()
var webPlugins = l9plugins.GetWebPlugins()

var TcpPlugins []l9format.ServicePluginInterface
var WebPlugins []l9format.WebPluginInterface

func LoadL9ExplorePlugins(pluginsJson string) {

	err := LoadPluginsFromFile(pluginsJson)
	if err == nil {
		// Successfully loaded plugin file, so we can now process the plugins
		return
	}

	TcpPlugins = append(TcpPlugins, tcpPlugins...)
	WebPlugins = append(WebPlugins, webPlugins...)
	// Add your plugins here
	TcpPlugins = append(TcpPlugins, l9_nuclei_plugin.NucleiPlugin{})
}

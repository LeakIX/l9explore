package l9explore

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/LeakIX/l9format"
	"github.com/PuerkitoBio/goquery"
	"github.com/gboddin/goccm"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type ExploreServiceCommand struct {
	MaxThreads          int                               `help:"Max threads" short:"t" default:"10"`
	OnlyLeak            bool                              `help:"Discards services events" short:"l"`
	OpenPlugins         []l9format.ServicePluginInterface `kong:"-"`
	ExplorePlugins      []l9format.ServicePluginInterface `kong:"-"`
	ExfiltratePlugins   []l9format.ServicePluginInterface `kong:"-"`
	HttpPlugins         []l9format.WebPluginInterface              `kong:"-"`
	ThreadManager       *goccm.ConcurrencyManager         `kong:"-"`
	JsonEncoder         *json.Encoder                     `kong:"-"`
	JsonDecoder         *json.Decoder                     `kong:"-"`
	ExploreTimeout      time.Duration                     `short:"x" default:"3s"`
	DisableExploreStage bool                              `short:"e"`
	ExfiltrateStage     bool                              `short:"x"`
	Option              map[string]string                 `short:"o"`
	Debug               bool
	HttpRequests        map[string]l9format.WebPluginRequest `kong:"-"`
}

func (cmd *ExploreServiceCommand) Run() error {
	LoadL9ExplorePlugins()
	cmd.HttpRequests = make(map[string]l9format.WebPluginRequest)
	if !cmd.Debug {
		log.SetOutput(ioutil.Discard)
	}
	cmd.JsonDecoder = json.NewDecoder(os.Stdin)
	cmd.JsonEncoder = json.NewEncoder(os.Stdout)
	err := cmd.LoadPlugins()
	if err != nil {
		return err
	}
	cmd.ThreadManager = goccm.New(cmd.MaxThreads)
	defer cmd.ThreadManager.WaitAllDone()
	for {
		event := l9format.L9Event{}
		err = cmd.JsonDecoder.Decode(&event)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Fatal(err)
		}
		event.AddSource("l9explore")
		cmd.ThreadManager.Wait()
		go func(event l9format.L9Event) {
			defer cmd.ThreadManager.Done()
			event.Time = time.Now()
			// Run open stage, gather credentials, service info
			cmd.RunPlugin(&event, cmd.OpenPlugins)
			if event.Leak.Stage == "open" && !cmd.DisableExploreStage {
				// Run explore stage, reuse credentials to get more informations
				cmd.RunPlugin(&event, cmd.ExplorePlugins)
			}
			if (event.Leak.Stage == "explore" || event.Leak.Stage == "open") && cmd.ExfiltrateStage {
				// Run exfiltrate stage, dump parts or all data to filesystem
				cmd.RunPlugin(&event, cmd.ExfiltratePlugins)
			}
			if event.HasTransport("http") {
				cmd.RunWebPlugin(&event, cmd.HttpPlugins)
			}
			event.UpdateFingerprint()
			if !cmd.OnlyLeak {
				cmd.JsonEncoder.Encode(&event)
			}
		}(event)
	}
	return nil
}

func (cmd *ExploreServiceCommand) RunPlugin(event *l9format.L9Event, plugins []l9format.ServicePluginInterface) {
	// send to open plugins
	for _, loadedPlugin := range plugins {
		if event.MatchServicePlugin(loadedPlugin) {
			leakEvent := *event
			leakEvent.Summary = ""
			ctx, contextCancelFunc := context.WithTimeout(context.Background(), cmd.ExploreTimeout)
			hasLeak := loadedPlugin.Run(ctx, &leakEvent, cmd.Option)
			contextCancelFunc()
			if hasLeak {
				leakEvent.EventType = "leak"
				leakEvent.Leak.Stage, event.Leak.Stage = loadedPlugin.GetStage(), loadedPlugin.GetStage()
				leakEvent.AddSource(loadedPlugin.GetName())
				leakEvent.UpdateFingerprint()
				cmd.JsonEncoder.Encode(leakEvent)
			}
			if len(event.Service.Software.Name) < len(leakEvent.Service.Software.Name) {
				event.Service.Software = leakEvent.Service.Software
			}
			if len(event.SSH.Fingerprint) < len(leakEvent.SSH.Fingerprint) {
				event.SSH = leakEvent.SSH
			}
			if len(event.Service.Credentials.Username) < len(leakEvent.Service.Credentials.Username) {
				event.Service.Software = leakEvent.Service.Software
			}
		}
	}
}

func (cmd *ExploreServiceCommand) GetHttpClient(ctx context.Context, ip string, port string) *http.Client {
	if strings.Contains(ip, ":") && !strings.Contains(ip, "[") {
		ip = fmt.Sprintf("[%s]", ip)
	}
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _ string, _ string) (net.Conn, error) {
				addr := ip + ":" + port
				return l9format.ServicePluginBase{}.DialContext(ctx, "tcp", addr)
			},
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxConnsPerHost:       2,
			DisableKeepAlives: true,
			ResponseHeaderTimeout: 2 * time.Second,
			ExpectContinueTimeout: 2 * time.Second,
		},
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (cmd *ExploreServiceCommand) RunWebPlugin(event *l9format.L9Event, plugins []l9format.WebPluginInterface) {
	// do each requests and verify responses
	timeout := time.Duration(int64(cmd.ExploreTimeout)*int64(len(plugins)))
	ctx, contextCancelFunc := context.WithTimeout(context.Background(), timeout)
	defer contextCancelFunc()
	httpClient := cmd.GetHttpClient(ctx, event.Ip,event.Port)
	defer httpClient.CloseIdleConnections()
	if event.Host == "" {
		event.Host = event.Ip
	}

	for _, request := range cmd.HttpRequests {
		event.Http.Url = request.Path
		req, err := http.NewRequest(request.Method, event.Url(), bytes.NewReader(request.Body))
		if err != nil {
			log.Fatal("wtf ?")
		}
		req.Header.Set("User-Agent", "l9explore/1.0.0")
		for headerName, headerValue := range request.Headers {
			req.Header.Set(headerName, headerValue)
		}
		response := l9format.WebPluginResponse{}
		response.Response, err = httpClient.Do(req)
		if err != nil {
			continue
		}
		response.Body, _ = ioutil.ReadAll(io.LimitReader(response.Response.Body, 512*1024))
		response.Document, _ = goquery.NewDocumentFromReader(bytes.NewReader(response.Body))
		response.Response.Body.Close()
		for _, loadedPlugin := range plugins {
			leakEvent := *event
			leakEvent.Summary = ""
			hasLeak := loadedPlugin.Verify(request, response, &leakEvent, cmd.Option)
			if hasLeak {
				leakEvent.EventType = "leak"
				leakEvent.Leak.Stage, event.Leak.Stage = loadedPlugin.GetStage(), loadedPlugin.GetStage()
				leakEvent.AddSource(loadedPlugin.GetName())
				leakEvent.UpdateFingerprint()
				cmd.JsonEncoder.Encode(leakEvent)
			}
			if len(event.Service.Software.Name) < len(leakEvent.Service.Software.Name) {
				event.Service.Software = leakEvent.Service.Software
			}
			if len(event.Service.Credentials.Username) < len(leakEvent.Service.Credentials.Username) {
				event.Service.Software = leakEvent.Service.Software
			}
		}
	}
}

func (cmd *ExploreServiceCommand) LoadPlugins() error {
	for _, tcpPlugin := range TcpPlugins {
		if tcpPlugin.GetStage() == "open" {
			cmd.OpenPlugins = append(cmd.OpenPlugins, tcpPlugin)
		} else if tcpPlugin.GetStage() == "explore" {
			cmd.ExplorePlugins = append(cmd.ExplorePlugins, tcpPlugin)
		} else if tcpPlugin.GetStage() == "exfiltrate" {
			cmd.ExplorePlugins = append(cmd.ExplorePlugins, tcpPlugin)
		} else {
			panic("l9explore only supports open, explore and exfiltrate stage")
		}
		majorVersion, minorVersion, patchVersion := tcpPlugin.GetVersion()
		log.Printf("Plugin %s %d.%d.%d loaded for protocols %s. Stage: %s",
			tcpPlugin.GetName(), majorVersion, minorVersion, patchVersion, strings.Join(tcpPlugin.GetProtocols(), ", "), tcpPlugin.GetStage())
		err := tcpPlugin.Init()
		if err != nil {
			return err
		}
	}
	for _, webPlugin := range WebPlugins {
			cmd.HttpPlugins = append(cmd.HttpPlugins, webPlugin)
			majorVersion, minorVersion, patchVersion := webPlugin.GetVersion()
			// Plugins can register requests, this ensure they only run once
			for _, request := range webPlugin.GetRequests() {
				log.Printf("Loaded request ID %x", request.GetHash())
				cmd.HttpRequests[request.GetHash()] = request
			}
			log.Printf("Web Plugin %s %d.%d.%d loaded. Stage: %s",
				webPlugin.GetName(), majorVersion, minorVersion, patchVersion, webPlugin.GetStage())
	}
	log.Printf("loaded %d service plugins and %d web plugins (%d requests)", len(cmd.OpenPlugins) + len(cmd.ExplorePlugins) + len(cmd.ExfiltratePlugins), len(cmd.HttpPlugins), len(cmd.HttpRequests))
	return nil
}

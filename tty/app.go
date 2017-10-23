package tty

import (
	"errors"
	"net/http"
	"sync"
	"text/template"

	"fmt"

	"github.com/braintree/manners"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/spf13/pflag"
)

type App struct {
	options *Options

	upgrader *websocket.Upgrader
	server   *manners.GracefulServer

	titleTemplate *template.Template
}

type Options struct {
	Port            int
	IndexFile       string
	TitleFormat     string
	EnableReconnect bool
	ReconnectTime   int
	Preferences     HtermPrefernces
	RawPreferences  map[string]interface{}
	Address         string
}

func NewOptions() *Options {
	return &Options{
		IndexFile:       "",
		TitleFormat:     "TTY - {{ .Command }}",
		EnableReconnect: false,
		ReconnectTime:   10,
		Preferences:     HtermPrefernces{},
	}
}

func (tty *Options) AddFlag(fs *pflag.FlagSet) {
	fs.StringVar(&tty.Address, "address", "0.0.0.0", "listen  address")
	fs.IntVar(&tty.Port, "port", 8080, "listen port")
}

func New(options *Options) (*App, error) {
	titleTemplate, err := template.New("title").Parse(options.TitleFormat)
	if err != nil {
		return nil, errors.New("Title format string syntax error")
	}

	return &App{
		options: options,
		upgrader: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			Subprotocols:    []string{"docker-tty"},
		},
		titleTemplate: titleTemplate,
	}, nil
}

func (app *App) Run() error {

	staticHandler := http.FileServer(http.Dir("static/"))

	var siteMux = http.NewServeMux()

	siteMux.Handle("/", http.StripPrefix("/", staticHandler))
	siteMux.Handle("/js/", http.StripPrefix("/", staticHandler))
	siteMux.Handle("/favicon.png", http.StripPrefix("/", staticHandler))
	siteMux.HandleFunc("/ws", app.handleWS)

	app.server = manners.NewWithServer(
		&http.Server{
			Addr:    fmt.Sprintf("%s:%d", app.options.Address, app.options.Port),
			Handler: siteMux,
		},
	)

	if err := app.server.ListenAndServe(); err != nil {
		return err
	}

	glog.V(5).Info("Exiting...")

	return nil
}

func (app *App) handleWS(w http.ResponseWriter, r *http.Request) {

	glog.V(3).Infof("New client connected: %s", r.RemoteAddr)
	_ = r.URL.Query().Get("id")
	r.Header.Del("Origin")
	// get info from url, if use docker driver, format url
	kubeNamespace := "default"
	kubePod := "nginx-2489652730-rtvsg"
	kubeContainer := "nginx1"
	KubeApi := "10.1.4.159:8080"

	conn, err := app.upgrader.Upgrade(w, r, nil)
	if err != nil {
		glog.Warningf("Failed to upgrade connection: %s", err.Error())
		return
	}

	app.server.StartRoutine()

	context := &clientContext{
		app:        app,
		request:    r,
		connection: conn,
		writeMutex: &sync.Mutex{},
	}

	context.goHandleClient(kubeNamespace, kubePod, kubeContainer, KubeApi)
}

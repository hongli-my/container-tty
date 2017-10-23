package tty

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"strings"
	"sync"

	"github.com/fatih/structs"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
)

type clientContext struct {
	app        *App
	request    *http.Request
	connection *websocket.Conn

	writeMutex *sync.Mutex
}

const (
	Input          = '0'
	Ping           = '1'
	ResizeTerminal = '2'
)

const (
	Output         = '0'
	Pong           = '1'
	SetWindowTitle = '2'
	SetPreferences = '3'
	SetReconnect   = '4'
)

type ContextVars struct {
	Command string
}

func (context *clientContext) goHandleClient(kubeNamespace, kubePod, kubeContainer, KubeApi string) {
	exit := make(chan bool, 2)

	stdinPipeReader, stdinPipeWriter := io.Pipe()
	stdoutPipeReader, stdoutPipeWriter := io.Pipe()
	_, stderrPipeWriter := io.Pipe()

	newDriver := NewKubeExec(stdinPipeReader, stdoutPipeWriter, stderrPipeWriter,
		kubeNamespace, kubePod, kubeContainer, KubeApi)
	// TODO: format params when use docker driver
	//newDriver := NewDockerExec(stdinPipeReader, stdoutPipeWriter, stderrPipeWriter)

	go func() {
		defer func() { exit <- true }()

		context.processSend(stdoutPipeReader)
	}()

	go func() {
		defer func() { exit <- true }()

		context.processReceive(stdinPipeWriter)
	}()

	newDriver.Run()
	stdoutPipeWriter.Close()
	stderrPipeWriter.Close()
	stdinPipeReader.Close()
}

func (context *clientContext) processSend(sessionReader io.ReadCloser) {
	if err := context.sendInitialize(); err != nil {
		glog.Warning(err.Error())
		return
	}

	buf := make([]byte, 1024)

	for {
		size, err := sessionReader.Read(buf)
		if err != nil {
			glog.V(3).Infof("Command exited for: %s", context.request.RemoteAddr)
			return
		}
		safeMessage := base64.StdEncoding.EncodeToString([]byte(buf[:size]))
		if err = context.write(append([]byte{Output}, []byte(safeMessage)...)); err != nil {
			glog.Warningf("send message error: %s", err.Error())
			return
		}
	}
}

func (context *clientContext) write(data []byte) error {
	context.writeMutex.Lock()
	defer context.writeMutex.Unlock()
	return context.connection.WriteMessage(websocket.TextMessage, data)
}

func (context *clientContext) sendInitialize() error {
	titleVars := ContextVars{
		Command: "/bin/bash",
	}

	titleBuffer := new(bytes.Buffer)
	if err := context.app.titleTemplate.Execute(titleBuffer, titleVars); err != nil {
		return err
	}
	if err := context.write(append([]byte{SetWindowTitle}, titleBuffer.Bytes()...)); err != nil {
		return err
	}

	prefStruct := structs.New(context.app.options.Preferences)
	prefMap := prefStruct.Map()
	htermPrefs := make(map[string]interface{})
	for key, value := range prefMap {
		rawKey := prefStruct.Field(key).Tag("hcl")
		if _, ok := context.app.options.RawPreferences[rawKey]; ok {
			htermPrefs[strings.Replace(rawKey, "_", "-", -1)] = value
		}
	}
	prefs, err := json.Marshal(htermPrefs)
	if err != nil {
		return err
	}

	if err := context.write(append([]byte{SetPreferences}, prefs...)); err != nil {
		return err
	}
	if context.app.options.EnableReconnect {
		reconnect, _ := json.Marshal(context.app.options.ReconnectTime)
		if err := context.write(append([]byte{SetReconnect}, reconnect...)); err != nil {
			return err
		}
	}
	return nil
}

func (context *clientContext) processReceive(sessionWriter io.WriteCloser) {

	for {
		_, data, err := context.connection.ReadMessage()
		if err != nil {
			glog.Info(err.Error())
			// Fix BUG: not input exit before close web page, left process in container, waster resources
			sessionWriter.Write([]byte("exit\r"))
			return
		}
		if len(data) == 0 {
			glog.Warning("An error has occured")
			return
		}
		switch data[0] {
		case Input:
			_, err := sessionWriter.Write(data[1:])
			if err != nil {
				return
			}

		case Ping:
			if err := context.write([]byte{Pong}); err != nil {
				glog.Warningf("Send Ping Message error: %s", err.Error())
				return
			}
		case ResizeTerminal:
			// TODO: Resize Terminal
			glog.V(3).Info("resize terminal, TODO in next term")

		default:
			glog.Warning("Unknown message type")
			return
		}
	}
}

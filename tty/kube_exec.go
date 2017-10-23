package tty

import (
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoapi "k8s.io/client-go/pkg/api"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api"

	"net/url"

	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/client/unversioned/remotecommand"
	remotecommandserver "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"
	"k8s.io/kubernetes/pkg/util/interrupt"
	"k8s.io/kubernetes/pkg/util/term"
)

type RemoteExecutor interface {
	Execute(method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool, terminalSizeQueue term.TerminalSizeQueue) error
}

type DefaultRemoteExecutor struct{}

func (*DefaultRemoteExecutor) Execute(method string, url *url.URL, config *restclient.Config, stdin io.Reader, stdout, stderr io.Writer, tty bool, terminalSizeQueue term.TerminalSizeQueue) error {
	exec, err := remotecommand.NewExecutor(config, method, url)
	if err != nil {
		fmt.Println(err)
		return err
	}
	return exec.Stream(remotecommand.StreamOptions{
		SupportedProtocols: remotecommandserver.SupportedStreamingProtocols,
		Stdin:              stdin,
		Stdout:             stdout,
		Stderr:             stderr,
		Tty:                tty,
		TerminalSizeQueue:  terminalSizeQueue,
	})
}

func NewKubeExec(cmdIn io.ReadCloser, cmdOut, cmdErr io.WriteCloser,
	kubeNamespace, kubePod, kubeContainer, KubeApi string) *ExecOptions {
	options := &ExecOptions{
		StreamOptions: StreamOptions{
			In:    cmdIn,
			Out:   cmdOut,
			Err:   cmdErr,
			Stdin: true,
			TTY:   true,
		},
		Executor: &DefaultRemoteExecutor{},
	}
	options.Config = &restclient.Config{
		Host: fmt.Sprintf("http://%s/api", KubeApi),
		ContentConfig: restclient.ContentConfig{
			GroupVersion: &schema.GroupVersion{
				Group:   "",
				Version: "v1",
			},
			NegotiatedSerializer: clientgoapi.Codecs,
		},
	}
	options.PodName = kubePod
	options.ContainerName = kubeContainer
	options.Namespace = kubeNamespace
	options.Command = []string{"env", fmt.Sprintf("TERM=%s", "xterm-256color"), "/bin/bash"}
	return options
}

type StreamOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         bool
	TTY           bool
	// minimize unnecessary output
	Quiet bool
	// InterruptParent, if set, is used to handle interrupts while attached
	InterruptParent *interrupt.Handler
	In              io.ReadCloser
	Out             io.WriteCloser
	Err             io.WriteCloser

	// for testing
	overrideStreams func() (io.ReadCloser, io.WriteCloser, io.WriteCloser)
	isTerminalIn    func(t term.TTY) bool
}

type ExecOptions struct {
	StreamOptions

	Command []string

	FullCmdName       string
	SuggestedCmdUsage string

	Executor  RemoteExecutor
	PodClient coreclient.PodsGetter
	Config    *restclient.Config
}

func (p *ExecOptions) Run() error {
	// TODO: check pod or container exist

	var sizeQueue term.TerminalSizeQueue

	fn := func() error {
		restClient, err := restclient.RESTClientFor(p.Config)
		if err != nil {
			return err
		}

		req := restClient.Post().
			Resource("pods").
			Name(p.PodName).
			Namespace(p.Namespace).
			SubResource("exec").
			Param("container", p.ContainerName)
		req.VersionedParams(&api.PodExecOptions{
			Container: p.ContainerName,
			Command:   p.Command,
			Stdin:     p.Stdin,
			Stdout:    p.Out != nil,
			Stderr:    p.Err != nil,
			TTY:       true,
		}, api.ParameterCodec)
		return p.Executor.Execute("POST", req.URL(), p.Config, p.In, p.Out, p.Err, true, sizeQueue)
	}
	fn()

	return nil
}

package tty

import (
	"fmt"
	"io"
	"net"

	"github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
)

type DockerExec struct {
	dockerClient     *docker.Client
	stdoutPipeWriter io.WriteCloser
	stderrPipeWriter io.WriteCloser
	stdinPipeReader  io.ReadCloser
	exec             *docker.Exec
}

func NewDockerExec(cmdIn io.ReadCloser, cmdOut, cmdErr io.WriteCloser) *DockerExec {
	// TODO: docker remote api Maybe get from front web, so can connect many backend
	// also include uid
	var err error
	client, _ := docker.NewClient(net.JoinHostPort("127.0.0.1", "2375"))
	dockerExec := &DockerExec{
		dockerClient:     client,
		stdoutPipeWriter: cmdOut,
		stderrPipeWriter: cmdErr,
		stdinPipeReader:  cmdIn,
	}
	execCmd := []string{"env", fmt.Sprintf("TERM=%s", "xterm-256color"), "/bin/bash"}

	opts := docker.CreateExecOptions{
		Container:    "agitated_wozniak", // uid or name
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          execCmd,
	}
	dockerExec.exec, err = dockerExec.dockerClient.CreateExec(opts)
	if err != nil {
		glog.Warningf("Create Docker Exec err: %s", err.Error())
		return nil
	}
	return dockerExec
}

func (d *DockerExec) Run() {

	if err := d.dockerClient.StartExec(d.exec.ID, docker.StartExecOptions{
		Detach:       false,
		OutputStream: d.stdoutPipeWriter,
		ErrorStream:  d.stderrPipeWriter,
		InputStream:  d.stdinPipeReader,
		RawTerminal:  false,
	}); err != nil {
		glog.Warningf("Start exec err %s", err.Error())
	}
}

package transport

import (
	"bytes"
	"context"
	"io"
	"sync"
)

// This is the phrase that tells us sudo is looking for a password via stdin
const sudoPwPrompt = "sudo_password"

// sudoWriter is used to both combine stdout and stderr as well as
// look for a password request from sudo.
type sudoWriter struct {
	b     bytes.Buffer
	pw    string    // The password to pass to sudo (if requested)
	stdin io.Writer // The writer from the ssh session
	m     sync.Mutex
}

func (w *sudoWriter) Write(p []byte) (int, error) {
	// If we get the sudo password prompt phrase send the password via stdin
	// and don't write it to the buffer.
	if string(p) == sudoPwPrompt {
		w.stdin.Write([]byte(w.pw + "\n"))
		w.pw = "" // We don't need the password anymore so reset the string
		return len(p), nil
	} else if string(p) == "\r\n" || string(p) == "\n" || string(p) == "\r" {
		return len(p), nil
	}

	w.m.Lock()
	defer w.m.Unlock()

	return w.b.Write(p)
}

func (s *Session) Sudo(cmd, passwd string) ([]byte, error) {
	if cmd == "" {
		return s.Output(cmd)
	}
	cmd = "sudo -p " + sudoPwPrompt + " -S " + cmd

	// Use the sudoRW struct to handle the interaction with sudo and capture the
	// output of the command
	w := &sudoWriter{
		pw: passwd,
	}
	w.stdin = s.stdin

	s.SetStderr(w)
	s.SetStdout(w)

	err := s.Run(cmd)

	return w.b.Bytes(), err
}

func (s *Session) SudoContext(ctx context.Context, cmd, passwd string) ([]byte, error) {
	quit := make(chan struct{}, 1)
	defer close(quit)

	go func() {
		defer s.Close()
		select {
		case <-ctx.Done():
			return
		case <-quit:
			return
		}
	}()

	if cmd == "" {
		return s.Output(cmd)
	}
	cmd = "sudo -p " + sudoPwPrompt + " -S " + cmd

	// Use the sudoRW struct to handle the interaction with sudo and capture the
	// output of the command
	w := &sudoWriter{
		pw: passwd,
	}
	w.stdin = s.stdin

	s.SetStderr(w)
	s.SetStdout(w)

	err := s.Run(cmd)

	return w.b.Bytes(), err
}

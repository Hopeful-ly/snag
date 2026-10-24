package ipc

import (
	"errors"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

// Listen opens addr, a named pipe, for connections from the current user only.
func Listen(addr string) (net.Listener, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	l, err := winio.ListenPipe(addr, &winio.PipeConfig{
		SecurityDescriptor: "D:P(A;;GA;;;" + user.User.Sid.String() + ")",
	})
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_PIPE_BUSY) {
		return nil, ErrInUse
	}
	return l, err
}

// Dial connects to the app listening on addr; a timeout of 0 waits the pipe library's default.
func Dial(addr string, timeout time.Duration) (net.Conn, error) {
	if timeout <= 0 {
		return winio.DialPipe(addr, nil)
	}
	return winio.DialPipe(addr, &timeout)
}

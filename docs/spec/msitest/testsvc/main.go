// Disposable test service for MSI ServiceInstall/ServiceControl tests.
package main

import (
	"time"

	"golang.org/x/sys/windows/svc"
)

type handler struct{}

func (handler) Execute(_ []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	s <- svc.Status{State: svc.StartPending}
	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			s <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			s <- svc.Status{State: svc.StopPending}
			time.Sleep(100 * time.Millisecond)
			return false, 0
		}
	}
	return false, 0
}

func main() {
	_ = svc.Run("MsiSpecTestSvc", handler{})
}

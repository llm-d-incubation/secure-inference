package runnable

import (
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// noLeaderElection wraps a Runnable so it runs on ALL replicas,
// not just the leader. Used for the ext-auth gRPC server which must
// serve traffic on every replica.
type noLeaderElection struct {
	manager.Runnable
}

// NeedLeaderElection returns false so this runnable runs on all replicas.
func (n *noLeaderElection) NeedLeaderElection() bool {
	return false
}

// NoLeaderElection wraps a Runnable to indicate it should run on all replicas.
func NoLeaderElection(r manager.Runnable) manager.Runnable {
	return &noLeaderElection{Runnable: r}
}

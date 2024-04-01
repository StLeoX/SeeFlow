package bgtask

// BgTaskManager manages background periodical tasks.
// Includes:
// - Sync t_Ep table
// - Sync namespace list
// - Run Assemble algorithm
type BgTaskManager struct {
}

func NewBgTaskManager() {
	NewNamespaceWatcher()

}

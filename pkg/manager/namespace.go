package manager

type namespaceBackupManager struct {
	BackupOptions
}

func newNamespaceBackupManager(opt BackupOptions) BackupManager {
	return namespaceBackupManager{
		BackupOptions: opt,
	}
}

func (opt namespaceBackupManager) Dump() error {
	return nil
}

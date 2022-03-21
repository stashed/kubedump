package manager

type applicationBackupManager struct {
	BackupOptions
}

func newApplicationBackupManager(opt BackupOptions) BackupManager {
	return applicationBackupManager{
		BackupOptions: opt,
	}
}

func (opt applicationBackupManager) Dump() error {
	return nil
}

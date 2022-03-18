package manifests

import "kmodules.xyz/client-go/tools/backup"

type clusterDumper struct {
}

func newClusterDumper() ManifestDumper {
	return clusterDumper{}
}

func (opt clusterDumper) Dump() error {
	// backup cluster resources yaml into opt.backupDir
	mgr := backup.NewBackupManager(opt.context, opt.config, opt.sanitize)

	_, err := mgr.BackupToDir(opt.interimDataDir)
	if err != nil {
		return err
	}
	return nil
}

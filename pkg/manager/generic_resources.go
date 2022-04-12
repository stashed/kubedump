package manager

import (
	"path/filepath"
	"strings"

	"stash.appscode.dev/apimachinery/apis"
	"stash.appscode.dev/manifest-backup/pkg/sanitizers"

	"gomodules.xyz/sets"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

type genericResourceBackupManager struct {
	namespace      string
	storage        Writer
	config         *rest.Config
	sanitize       bool
	dataDir        string
	selector       string
	useRootDataDir bool
}

func newGenericResourceBackupManager(opt BackupOptions) BackupManager {
	mgr := genericResourceBackupManager{
		config:   opt.Config,
		storage:  opt.Storage,
		sanitize: opt.Sanitize,
		dataDir:  opt.DataDir,
		selector: opt.Selector,
	}
	if opt.Target.Kind == apis.KindNamespace {
		mgr.namespace = opt.Target.Name
		mgr.useRootDataDir = true
	}
	return mgr
}

func (opt genericResourceBackupManager) Dump() error {
	processor := itemDumper{
		sanitize:       opt.sanitize,
		dataDir:        opt.dataDir,
		storage:        opt.storage,
		useRootDataDir: opt.useRootDataDir,
	}

	rp := resourceProcessor{
		config:        opt.config,
		namespace:     opt.namespace,
		selector:      opt.selector,
		itemProcessor: processor,
	}
	return rp.processAPIResources()
}

type itemDumper struct {
	sanitize       bool
	dataDir        string
	storage        Writer
	useRootDataDir bool
}

func (opt itemDumper) Process(items []unstructured.Unstructured, _ schema.GroupVersionResource) error {
	var err error
	for _, r := range items {
		data := r.Object
		if opt.sanitize {
			s := sanitizers.NewSanitizer(r.GetKind())
			data, err = s.Sanitize(data)
			if err != nil {
				return err
			}
			delete(data, "status")
		}

		fileName := opt.getFileName(r)
		err = storeItem(fileName, data, opt.storage)
		if err != nil {
			return err
		}
	}
	return nil
}

func (opt *itemDumper) getFileName(r unstructured.Unstructured) string {
	if opt.useRootDataDir {
		return filepath.Join(opt.dataDir, r.GetKind(), r.GetName()) + ".yaml"
	}

	prefix := ""
	if r.GetNamespace() != "" {
		prefix = filepath.Join(opt.dataDir, "namespaces", r.GetNamespace())
	} else {
		prefix = filepath.Join(opt.dataDir, "global")
	}
	return filepath.Join(prefix, r.GetKind(), r.GetName()) + ".yaml"
}

func storeItem(fileName string, in map[string]interface{}, storage Writer) error {
	data, err := yaml.Marshal(in)
	if err != nil {
		return err
	}
	err = storage.Write(fileName, data)
	if err != nil {
		return err
	}
	return nil
}

func isSubResource(name string) bool {
	return strings.ContainsRune(name, '/')
}

func hasGetListVerbs(verbs []string) bool {
	return sets.NewString(verbs...).HasAll("get", "list")
}

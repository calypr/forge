package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/calypr/forge/utils/remoteutil"
)

func RunConfigInit(remoteName string) error {
	remoteConfig, err := remoteutil.LoadRemoteOrDefault(remoteName)
	if err != nil {
		return err
	}
	if remoteConfig.ProjectID == "" {
		return fmt.Errorf("projectID is empty for remote %s", remoteConfig.Name)
	}

	err = os.MkdirAll("CONFIG", os.ModePerm)
	if err != nil {
		return err
	}

	filePath := filepath.Join("CONFIG", remoteConfig.ProjectID+".json")
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("configuration file '%s' already exists. Aborting to prevent overwrite", filePath)
		}
		return err
	}
	defer f.Close()

	// Keep this template local so Forge does not need the Gecko module just to
	// scaffold a starter config file.
	emptyConf := map[string]any{
		"sharedFilters": map[string]any{
			"sharedFilter": map[string]any{
				"": []map[string]string{{
					"index": "",
					"field": "",
				}},
			},
		},
		"explorerConfig": []map[string]any{{
			"tabTitle": "",
			"filters": map[string]any{
				"tabs": []map[string]any{{
					"title":  "",
					"fields": []string{},
					"fieldsConfig": map[string]any{
						"": map[string]string{
							"field":     "",
							"dataField": "",
							"index":     "",
							"label":     "",
							"type":      "",
						},
					},
				}},
			},
			"charts": map[string]any{
				"": map[string]string{
					"chartType": "",
					"title":     "",
				},
			},
			"guppyConfig": map[string]string{
				"dataType": "",
			},
			"table": map[string]any{
				"enabled": false,
				"fields":  []string{},
				"columns": map[string]any{
					"": map[string]string{
						"field":        "",
						"title":        "",
						"accessorPath": "",
					},
				},
			},
			"dropdowns":        map[string]any{},
			"loginForDownload": false,
		}},
	}

	mEConf, err := json.Marshal(emptyConf)
	if err != nil {
		return err
	}

	_, err = f.Write(mEConf)
	return nil
}

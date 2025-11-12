package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	gconf "github.com/calypr/gecko/gecko/config"
	"github.com/calypr/git-drs/config"
)

func RunConfigInit() error {
	conf, err := config.LoadConfig()
	if err != nil {
		return err
	}
	if conf.Servers.Gen3 == nil {
		return fmt.Errorf("Config generation expects a populated gen3 config but conf.Servers.Gen3 is nil")
	}

	projectId := conf.Servers.Gen3.Auth.ProjectID
	err = os.MkdirAll("CONFIG", os.ModePerm)
	if err != nil {
		return err
	}

	filePath := filepath.Join("CONFIG", projectId+".json")
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("configuration file '%s' already exists. Aborting to prevent overwrite", filePath)
		}
		return err
	}
	defer f.Close()

	// Create an empty config so that users have the basic structure of a config
	// as a template for them so that they can focus more time on filling out hte config and less
	// time figuring out the correct structure of the config
	var emptyConf gconf.Config = gconf.Config{
		SharedFilters: gconf.SharedFiltersConfig{
			SharedFilter: map[string][]gconf.FilterPair{
				"": {
					{
						Index: "",
						Field: "",
					},
				},
			},
		},
		ExplorerConfig: []gconf.ConfigItem{{
			TabTitle: "",
			Filters: gconf.FiltersConfig{
				Tabs: []gconf.FilterTab{{
					Title:  "",
					Fields: []string{},
					FieldsConfig: map[string]gconf.FieldConfig{
						"": {
							Field:     "",
							DataField: "",
							Index:     "",
							Label:     "",
							Type:      "",
						},
					},
				}},
			},
			Charts: map[string]gconf.Chart{
				"": {
					ChartType: "",
					Title:     "",
				},
			},
			GuppyConfig: gconf.GuppyConfig{
				DataType: "",
			},
			Table: gconf.TableConfig{
				Enabled: false,
				Fields:  []string{},
				Columns: map[string]gconf.TableColumnsConfig{
					"": {
						Field:        "",
						Title:        "",
						AccessorPath: "",
					},
				},
			},
			Dropdowns:        map[string]any{},
			LoginForDownload: false,
		}},
	}

	mEConf, err := json.Marshal(emptyConf)
	if err != nil {
		return err
	}

	_, err = f.Write(mEConf)
	return nil
}

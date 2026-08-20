package internal

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var m49SourcePaths = []string{
	"cldr/codeMappings.json",
	"cldr/en-territories.json",
	"cldr/zh-territories.json",
	"cldr/supplementalData.xml",
}

type supplementalData struct {
	TerritoryContainment struct {
		Groups []territoryGroup `xml:"group"`
	} `xml:"territoryContainment"`
}

type territoryGroup struct {
	Type     string `xml:"type,attr"`
	Contains string `xml:"contains,attr"`
	Status   string `xml:"status,attr"`
	Grouping bool   `xml:"grouping,attr"`
}

// GenerateM49 在区域生成前生成 M.49 层级和 ISO 区域索引。
func GenerateM49(root string, states map[string]SourceState) (map[string]M49Index, bool, error) {
	files, err := sourceFiles(states, m49SourcePaths)
	if err != nil {
		return nil, false, err
	}
	node, indexes, err := parseM49(files)
	if err != nil {
		return nil, false, err
	}
	body, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return nil, false, err
	}
	body = append(body, '\n')
	path := filepath.Join(root, "generated", "m49.json")
	current, err := os.ReadFile(path)
	if err == nil && bytes.Equal(current, body) {
		return indexes, false, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, false, fmt.Errorf("read generated M.49: %w", err)
	}
	if err := writeFile(filepath.Join(root, "generated"), "m49.json", body); err != nil {
		return nil, false, err
	}
	return indexes, true, nil
}

func parseM49(files map[string][]byte) (*M49Node, map[string]M49Index, error) {
	var mappings codeMappingsFile
	var en, zh territoryNamesFile
	var data supplementalData
	if err := json.Unmarshal(files["cldr/codeMappings.json"], &mappings); err != nil {
		return nil, nil, fmt.Errorf("parse codeMappings: %w", err)
	}
	if err := json.Unmarshal(files["cldr/en-territories.json"], &en); err != nil {
		return nil, nil, fmt.Errorf("parse English territories: %w", err)
	}
	if err := json.Unmarshal(files["cldr/zh-territories.json"], &zh); err != nil {
		return nil, nil, fmt.Errorf("parse Chinese territories: %w", err)
	}
	if err := xml.Unmarshal(files["cldr/supplementalData.xml"], &data); err != nil {
		return nil, nil, fmt.Errorf("parse supplementalData: %w", err)
	}

	groups := regularGroups(data.TerritoryContainment.Groups)
	enNames, zhNames := namesFor(en, "en"), namesFor(zh, "zh")
	indexes := make(map[string]M49Index)
	root, err := buildM49Node("001", groups, mappings.Supplemental.CodeMappings, enNames, zhNames, indexes, M49Index{})
	if err != nil {
		return nil, nil, err
	}
	for iso, mapping := range mappings.Supplemental.CodeMappings {
		if !isISORegion(iso, mapping) {
			continue
		}
		if _, exists := indexes[iso]; exists {
			continue
		}
		indexes[iso] = M49Index{}
		root.Regions = append(root.Regions, iso)
	}
	sort.Strings(root.Regions)
	return root, indexes, nil
}

func regularGroups(groups []territoryGroup) map[string][]string {
	result := make(map[string][]string)
	for _, group := range groups {
		if group.Status != "" || group.Grouping || !validM49(group.Type) {
			continue
		}
		result[group.Type] = append(result[group.Type], strings.Fields(group.Contains)...)
	}
	return result
}
func buildM49Node(
	code string,
	groups map[string][]string,
	mappings map[string]codeMapping,
	enNames, zhNames map[string]string,
	indexes map[string]M49Index,
	parent M49Index,
) (*M49Node, error) {
	name := enNames[code]
	if name == "" {
		return nil, fmt.Errorf("missing English M.49 name for %s", code)
	}
	node := &M49Node{Code: code, Names: Names{English: name, Chinese: zhNames[code]}}
	for _, child := range groups[code] {
		if validM49(child) {
			index := parent
			if code == "001" {
				index.Region = child
			} else if parent.Region != "" && parent.Subregion == "" {
				index.Subregion = child
			}
			childNode, err := buildM49Node(child, groups, mappings, enNames, zhNames, indexes, index)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, childNode)
			continue
		}
		mapping, exists := mappings[child]
		if !exists || !isISORegion(child, mapping) {
			continue
		}
		indexes[child] = parent
		node.Regions = append(node.Regions, child)
	}
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Names.English < node.Children[j].Names.English
	})
	sort.Strings(node.Regions)
	return node, nil
}

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

// GenerateM49 在依赖源更新时重新生成 M.49 层级，否则解析既有生成文件。
func GenerateM49(root string, states *States) (M49Generation, error) {
	if cached := states.M49(); cached != nil {
		return *cached, nil
	}

	result, err := generateM49(root, states)
	if err != nil {
		return M49Generation{}, err
	}
	states.SetM49(result)
	return result, nil
}

func generateM49(root string, states *States) (M49Generation, error) {
	if !states.SourcesUpdated(m49SourcePaths) {
		return readGeneratedM49(root)
	}

	files, err := states.SourceFiles(m49SourcePaths)
	if err != nil {
		return M49Generation{}, err
	}
	parsed, err := parseM49(files)
	if err != nil {
		return M49Generation{}, err
	}
	body, err := marshalGeneratedData(parsed.Root)
	if err != nil {
		return M49Generation{}, err
	}
	body = append(body, '\n')
	path := filepath.Join(root, "generated", "m49.json")
	current, err := os.ReadFile(path)
	if err == nil && bytes.Equal(current, body) {
		return M49Generation{Indexes: parsed.Indexes}, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return M49Generation{}, fmt.Errorf("read generated M.49: %w", err)
	}
	if err := writeFile(filepath.Join(root, "generated"), "m49.json", body); err != nil {
		return M49Generation{}, err
	}
	return M49Generation{Indexes: parsed.Indexes, Updated: true}, nil
}

func readGeneratedM49(root string) (M49Generation, error) {
	body, err := os.ReadFile(filepath.Join(root, "generated", "m49.json"))
	if err != nil {
		return M49Generation{}, fmt.Errorf("read generated M.49: %w", err)
	}
	var node M49Node
	if err := json.Unmarshal(body, &node); err != nil {
		return M49Generation{}, fmt.Errorf("parse generated M.49: %w", err)
	}
	indexes := make(map[string]M49Index)
	collectM49Indexes(&node, M49Index{}, indexes)
	return M49Generation{Indexes: indexes}, nil
}

func collectM49Indexes(node *M49Node, parent M49Index, indexes map[string]M49Index) {
	for _, iso := range node.Regions {
		indexes[iso] = parent
	}
	for _, child := range node.Children {
		index := parent
		if node.Code == "001" {
			index.Region = child.Code
		} else if parent.Region != "" && parent.Subregion == "" {
			index.Subregion = child.Code
		}
		collectM49Indexes(child, index, indexes)
	}
}

func parseM49(files map[string][]byte) (M49ParseResult, error) {
	var mappings codeMappingsFile
	var en, zh territoryNamesFile
	var data supplementalData
	if err := json.Unmarshal(files["cldr/codeMappings.json"], &mappings); err != nil {
		return M49ParseResult{}, fmt.Errorf("parse codeMappings: %w", err)
	}
	if err := json.Unmarshal(files["cldr/en-territories.json"], &en); err != nil {
		return M49ParseResult{}, fmt.Errorf("parse English territories: %w", err)
	}
	if err := json.Unmarshal(files["cldr/zh-territories.json"], &zh); err != nil {
		return M49ParseResult{}, fmt.Errorf("parse Chinese territories: %w", err)
	}
	if err := xml.Unmarshal(files["cldr/supplementalData.xml"], &data); err != nil {
		return M49ParseResult{}, fmt.Errorf("parse supplementalData: %w", err)
	}

	groups := regularGroups(data.TerritoryContainment.Groups)
	enNames, zhNames := namesFor(en, "en"), namesFor(zh, "zh")
	indexes := make(map[string]M49Index)
	root, err := buildM49Node("001", groups, mappings.Supplemental.CodeMappings, enNames, zhNames, indexes, M49Index{})
	if err != nil {
		return M49ParseResult{}, err
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
	return M49ParseResult{Root: root, Indexes: indexes}, nil
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

func buildM49Node(code string, groups map[string][]string, mappings map[string]codeMapping, enNames, zhNames map[string]string, indexes map[string]M49Index, parent M49Index) (*M49Node, error) {
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
	sort.Slice(node.Children, func(i, j int) bool { return node.Children[i].Names.English < node.Children[j].Names.English })
	sort.Strings(node.Regions)
	return node, nil
}

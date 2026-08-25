package internal

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lingting/lingting-geo-data/internal/cldr"
)

// GenerateM49 在 CLDR 更新或生成目标缺失时重新生成 M.49 层级。
func GenerateM49(root string, states *States) (M49Generation, error) {
	if cached, err := states.ReadM49(); err == nil {
		return cached, nil
	}
	data, err := states.ReadCLDR(root)
	if err != nil {
		return M49Generation{}, err
	}
	if !data.Updated && targetExists(filepath.Join(root, "generated", "m49.json")) {
		result, err := readGeneratedM49(root)
		if err != nil {
			return M49Generation{}, err
		}
		states.SetM49(result)
		return result, nil
	}
	parsed, err := parseM49(data)
	if err != nil {
		return M49Generation{}, err
	}
	updated, err := writeGeneratedJSON(root, "m49.json", parsed.Root)
	if err != nil {
		return M49Generation{}, err
	}
	result := M49Generation{Indexes: parsed.Indexes, Updated: updated}
	states.SetM49(result)
	return result, nil
}

func readGeneratedM49(root string) (M49Generation, error) {
	node, err := readJSON[M49Node](filepath.Join(root, "generated", "m49.json"))
	if err != nil {
		return M49Generation{}, fmt.Errorf("read generated M.49: %w", err)
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

func parseM49(data cldr.Data) (M49ParseResult, error) {
	groups := regularGroups(data.SupplementalData.Groups)
	indexes := make(map[string]M49Index)
	root, err := buildM49Node(
		"001",
		groups,
		data.CodeMappings.Mappings,
		data.EnglishTerritory.Names,
		data.ChineseTerritory.Names,
		indexes,
		M49Index{},
	)
	if err != nil {
		return M49ParseResult{}, err
	}
	for iso, mapping := range data.CodeMappings.Mappings {
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

func regularGroups(groups []cldr.TerritoryGroup) map[string][]string {
	result := make(map[string][]string)
	for _, group := range groups {
		if group.Status != "" || group.Grouping || !validM49(group.Type) {
			continue
		}
		result[group.Type] = append(result[group.Type], strings.Fields(group.Contains)...)
	}
	return result
}

func buildM49Node(code string, groups map[string][]string, mappings map[string]cldr.CodeMapping, enNames, zhNames map[string]string, indexes map[string]M49Index, parent M49Index) (*M49Node, error) {
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

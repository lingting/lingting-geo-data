package internal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func Sync(ctx context.Context, root string) (bool, error) {
	previous, err := readSourceIndex(root)
	if err != nil {
		return false, err
	}
	resources, err := downloadResources(ctx, root, baseSources(), previous)
	if err != nil {
		return false, err
	}
	states := newStates(resources)
	m49, err := GenerateM49(root, states)
	if err != nil {
		return false, err
	}
	phones, err := GeneratePhones(root, states)
	if err != nil {
		return false, err
	}
	regions, err := GenerateRegions(root, states)
	if err != nil {
		return false, err
	}
	if err := validateGeneratedData(root, m49, regions, phones); err != nil {
		return false, err
	}
	m49Updated, err := writeGeneratedJSON(root, "m49.json", m49.Root)
	if err != nil {
		return false, err
	}
	phonesUpdated, err := writeGeneratedJSON(root, "phones.json", flattenPhones(phones.Regions))
	if err != nil {
		return false, err
	}
	regionsUpdated, err := writeGeneratedJSON(root, "regions.json", regions.Regions)
	if err != nil {
		return false, err
	}
	if err := writeUpdatedResources(root, &resources); err != nil {
		return false, err
	}
	if err := removeLegacyCountries(root); err != nil {
		return false, err
	}
	flagsUpdated, err := GenerateFlags(root, states)
	if err != nil {
		return false, err
	}
	return resourcesUpdated(resources) || m49Updated || phonesUpdated || regionsUpdated || flagsUpdated, nil
}

func removeLegacyCountries(root string) error {
	err := os.Remove(filepath.Join(root, "generated", "countries.json"))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy countries: %w", err)
	}
	return nil
}

func readSourceIndex(root string) (SourceIndex, error) {
	body, err := os.ReadFile(filepath.Join(root, "generated", "sources.json"))
	if os.IsNotExist(err) {
		return SourceIndex{}, nil
	}
	if err != nil {
		return SourceIndex{}, err
	}
	var index SourceIndex
	if err := json.Unmarshal(body, &index); err != nil {
		return SourceIndex{}, fmt.Errorf("parse sources.json: %w", err)
	}
	return index, nil
}

func writeUpdatedResources(root string, resources *Resources) error {
	for _, resource := range resourceList(*resources) {
		record := resource.Record
		record.Path = resource.Spec.Path
		record.SHA256 = filesHash(resource.Files)
		if resource.Updated {
			for path, body := range resource.Files {
				if err := writePath(filepath.Join(root, "sources", filepath.FromSlash(path)), body); err != nil {
					return err
				}
			}
		}
		setResourceRecord(resources, resource.Spec.Path, record)
	}
	body, err := marshalJSON(resourcesIndex(*resources))
	if err != nil {
		return err
	}
	return writePath(filepath.Join(root, "generated", "sources.json"), append(body, '\n'))
}

func filesHash(files map[string][]byte) string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		digest := sha256.Sum256(files[path])
		_, _ = hash.Write([]byte(path + "\x00" + hex.EncodeToString(digest[:]) + "\n"))
	}
	return hex.EncodeToString(hash.Sum(nil))
}
func resourcesUpdated(resources Resources) bool {
	for _, resource := range resourceList(resources) {
		if resource.Updated {
			return true
		}
	}
	return false
}
func resourceList(resources Resources) []Resource {
	return []Resource{resources.CldrCodeMappings, resources.CldrEnTerritories, resources.CldrZhTerritories, resources.CldrSupplementalData, resources.PhoneNumberMetadata, resources.FlagIcons}
}
func resourcesIndex(r Resources) SourceIndex {
	return SourceIndex{r.CldrCodeMappings.Record, r.CldrEnTerritories.Record, r.CldrZhTerritories.Record, r.CldrSupplementalData.Record, r.PhoneNumberMetadata.Record, r.FlagIcons.Record}
}
func setResourceRecord(r *Resources, path string, record SourceRecord) {
	for _, resource := range []*Resource{&r.CldrCodeMappings, &r.CldrEnTerritories, &r.CldrZhTerritories, &r.CldrSupplementalData, &r.PhoneNumberMetadata, &r.FlagIcons} {
		if resource.Spec.Path == path {
			resource.Record = record
			return
		}
	}
}

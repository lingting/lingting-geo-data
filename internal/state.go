package internal

import (
	"fmt"

	"github.com/lingting/lingting-geo-data/internal/cldr"
)

// step 是具名生成步骤共享的泛型内存结果缓存。
type step[T any] struct {
	result *T
}

func (s *step[T]) Read() (T, bool) {
	var empty T
	if s.result == nil {
		return empty, false
	}
	return *s.result, true
}

func (s *step[T]) Set(result T) {
	s.result = &result
}

// States 保存具名资源与步骤结果的内存缓存。
type States struct {
	resources Resources
	m49       step[M49Generation]
	phones    step[PhoneGeneration]
	regions   step[RegionGeneration]
	cldr      step[cldr.Data]
}

func newStates(resources Resources) *States {
	return &States{resources: resources}
}

func (s *States) ReadCLDR(root string) (cldr.Data, error) {
	if cached, ok := s.cldr.Read(); ok {
		return cached, nil
	}
	resources := s.resources
	result, err := cldr.Read(
		root,
		resources.CldrCodeMappings.Files[resources.CldrCodeMappings.Spec.Path],
		resources.CldrEnTerritories.Files[resources.CldrEnTerritories.Spec.Path],
		resources.CldrZhTerritories.Files[resources.CldrZhTerritories.Spec.Path],
		resources.CldrSupplementalData.Files[resources.CldrSupplementalData.Spec.Path],
		resources.CldrCodeMappings.Updated ||
			resources.CldrEnTerritories.Updated ||
			resources.CldrZhTerritories.Updated ||
			resources.CldrSupplementalData.Updated,
	)
	if err != nil {
		return cldr.Data{}, err
	}
	s.cldr.Set(result)
	return result, nil
}

func (s *States) ReadM49() (M49Generation, error) {
	if result, ok := s.m49.Read(); ok {
		return result, nil
	}
	return M49Generation{}, fmt.Errorf("M.49 has not been generated")
}

func (s *States) ReadPhones() (PhoneGeneration, error) {
	if result, ok := s.phones.Read(); ok {
		return result, nil
	}
	return PhoneGeneration{}, fmt.Errorf("phones have not been generated")
}

func (s *States) ReadRegions() (RegionGeneration, error) {
	if result, ok := s.regions.Read(); ok {
		return result, nil
	}
	return RegionGeneration{}, fmt.Errorf("regions have not been generated")
}

func (s *States) ReadPhoneMetadata() Resource {
	return s.resources.PhoneNumberMetadata
}

func (s *States) ReadFlagIcons() Resource {
	return s.resources.FlagIcons
}

func (s *States) SetM49(result M49Generation) {
	s.m49.Set(result)
}

func (s *States) SetPhones(result PhoneGeneration) {
	s.phones.Set(result)
}

func (s *States) SetRegions(result RegionGeneration) {
	s.regions.Set(result)
}

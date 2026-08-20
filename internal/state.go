package internal

import "fmt"

// States 保存源文件状态及生成数据的内存缓存。
type States struct {
	sources map[string]SourceState
	m49     *M49Generation
	phones  *PhoneGeneration
	regions *RegionGeneration
}

func newStates(sources map[string]SourceState) *States {
	return &States{sources: sources}
}

func (s *States) Source(path string) (SourceState, error) {
	state, exists := s.sources[path]
	if !exists {
		return SourceState{}, fmt.Errorf("missing source %s", path)
	}
	return state, nil
}

func (s *States) Sources() map[string]SourceState {
	return s.sources
}

func (s *States) SourceFiles(paths []string) (map[string][]byte, error) {
	files := make(map[string][]byte, len(paths))
	for _, sourcePath := range paths {
		state, err := s.Source(sourcePath)
		if err != nil {
			return nil, err
		}
		for filePath, body := range state.Files {
			files[filePath] = body
		}
	}
	return files, nil
}

func (s *States) SourcesUpdated(paths []string) bool {
	for _, sourcePath := range paths {
		state, err := s.Source(sourcePath)
		if err == nil && state.Updated {
			return true
		}
	}
	return false
}

func (s *States) SourcesAreUpdated() bool {
	for _, state := range s.sources {
		if state.Updated {
			return true
		}
	}
	return false
}

func (s *States) M49() *M49Generation {
	return s.m49
}

func (s *States) SetM49(result M49Generation) {
	s.m49 = &result
}

func (s *States) Phones() *PhoneGeneration {
	return s.phones
}

func (s *States) SetPhones(result PhoneGeneration) {
	s.phones = &result
}

func (s *States) Regions() *RegionGeneration {
	return s.regions
}

func (s *States) SetRegions(result RegionGeneration) {
	s.regions = &result
}

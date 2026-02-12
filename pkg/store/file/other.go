package file

import (
	"context"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/store"
)

// recordingStore implements store.RecordingStore.
type recordingStore struct {
	fs *FileStore
}

func (s *recordingStore) List(ctx context.Context) ([]*store.Recording, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	result := make([]*store.Recording, len(s.fs.data.Recordings))
	copy(result, s.fs.data.Recordings)
	return result, nil
}

func (s *recordingStore) Get(ctx context.Context, id string) (*store.Recording, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()
	for _, r := range s.fs.data.Recordings {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, store.ErrNotFound
}

func (s *recordingStore) Create(ctx context.Context, recording *store.Recording) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}
	s.fs.data.Recordings = append(s.fs.data.Recordings, recording)
	s.fs.markDirty()
	return nil
}

func (s *recordingStore) Update(ctx context.Context, recording *store.Recording) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}
	for i, r := range s.fs.data.Recordings {
		if r.ID == recording.ID {
			s.fs.data.Recordings[i] = recording
			s.fs.markDirty()
			return nil
		}
	}
	return store.ErrNotFound
}

func (s *recordingStore) Delete(ctx context.Context, id string) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}
	for i, r := range s.fs.data.Recordings {
		if r.ID == id {
			s.fs.data.Recordings = append(s.fs.data.Recordings[:i], s.fs.data.Recordings[i+1:]...)
			s.fs.markDirty()
			return nil
		}
	}
	return store.ErrNotFound
}

func (s *recordingStore) DeleteAll(ctx context.Context) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}
	s.fs.data.Recordings = nil
	s.fs.markDirty()
	return nil
}

// requestLogStore implements store.RequestLogStore.
type requestLogStore struct {
	fs *FileStore
}

func (s *requestLogStore) List(ctx context.Context, limit, offset int) ([]*store.RequestLogEntry, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	entries := s.fs.data.RequestLog
	if offset >= len(entries) {
		return []*store.RequestLogEntry{}, nil
	}

	end := offset + limit
	if end > len(entries) || limit <= 0 {
		end = len(entries)
	}

	result := make([]*store.RequestLogEntry, end-offset)
	copy(result, entries[offset:end])
	return result, nil
}

func (s *requestLogStore) Get(ctx context.Context, id string) (*store.RequestLogEntry, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()
	for _, e := range s.fs.data.RequestLog {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, store.ErrNotFound
}

func (s *requestLogStore) Append(ctx context.Context, entry *store.RequestLogEntry) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}

	// Keep only last 10000 entries to prevent unbounded growth.
	// When evicting, compact the slice to a fresh backing array so that
	// repeated [1:] re-slices don't leak memory from the old array.
	const maxEntries = 10000
	if len(s.fs.data.RequestLog) >= maxEntries {
		compacted := make([]*store.RequestLogEntry, maxEntries-1, maxEntries)
		copy(compacted, s.fs.data.RequestLog[1:])
		s.fs.data.RequestLog = compacted
	}

	s.fs.data.RequestLog = append(s.fs.data.RequestLog, entry)
	s.fs.markDirty()
	return nil
}

func (s *requestLogStore) Clear(ctx context.Context) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}
	s.fs.data.RequestLog = nil
	s.fs.markDirty()
	return nil
}

func (s *requestLogStore) Count(ctx context.Context) (int, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()
	return len(s.fs.data.RequestLog), nil
}

// preferencesStore implements store.PreferencesStore.
type preferencesStore struct {
	fs *FileStore
}

func (s *preferencesStore) Get(ctx context.Context) (*store.Preferences, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()
	if s.fs.data.Preferences == nil {
		return &store.Preferences{
			Theme:            "system",
			AutoScroll:       true,
			PollingInterval:  2000,
			MinimizeToTray:   true,
			DefaultMockPort:  4280,
			DefaultAdminPort: 4290,
		}, nil
	}
	return s.fs.data.Preferences, nil
}

func (s *preferencesStore) Set(ctx context.Context, prefs *store.Preferences) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}
	s.fs.data.Preferences = prefs
	s.fs.markDirty()
	return nil
}

// folderStore implements store.FolderStore.
type folderStore struct {
	fs *FileStore
}

func (s *folderStore) List(ctx context.Context, filter *store.FolderFilter) ([]*config.Folder, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()

	// If no filter, return all folders
	if filter == nil {
		result := make([]*config.Folder, len(s.fs.data.Folders))
		copy(result, s.fs.data.Folders)
		return result, nil
	}

	// Filter folders
	var result []*config.Folder
	for _, f := range s.fs.data.Folders {
		// Filter by workspace
		// Treat empty workspaceID as "local" for backward compatibility
		if filter.WorkspaceID != "" {
			folderWsID := f.WorkspaceID
			if folderWsID == "" {
				folderWsID = store.DefaultWorkspaceID
			}
			if folderWsID != filter.WorkspaceID {
				continue
			}
		}
		// Filter by parent
		if filter.ParentID != nil {
			if *filter.ParentID == "" {
				// Root level only
				if f.ParentID != "" {
					continue
				}
			} else if f.ParentID != *filter.ParentID {
				continue
			}
		}
		result = append(result, f)
	}
	return result, nil
}

func (s *folderStore) Get(ctx context.Context, id string) (*config.Folder, error) {
	s.fs.mu.RLock()
	defer s.fs.mu.RUnlock()
	for _, f := range s.fs.data.Folders {
		if f.ID == id {
			return f, nil
		}
	}
	return nil, store.ErrNotFound
}

func (s *folderStore) Create(ctx context.Context, folder *config.Folder) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}
	s.fs.data.Folders = append(s.fs.data.Folders, folder)
	s.fs.markDirty()
	s.fs.notify("folders", "create", folder.ID, folder)
	return nil
}

func (s *folderStore) Update(ctx context.Context, folder *config.Folder) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}
	for i, f := range s.fs.data.Folders {
		if f.ID == folder.ID {
			s.fs.data.Folders[i] = folder
			s.fs.markDirty()
			s.fs.notify("folders", "update", folder.ID, folder)
			return nil
		}
	}
	return store.ErrNotFound
}

func (s *folderStore) Delete(ctx context.Context, id string) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}
	for i, f := range s.fs.data.Folders {
		if f.ID == id {
			s.fs.data.Folders = append(s.fs.data.Folders[:i], s.fs.data.Folders[i+1:]...)
			s.fs.markDirty()
			s.fs.notify("folders", "delete", id, nil)
			return nil
		}
	}
	return store.ErrNotFound
}

func (s *folderStore) DeleteAll(ctx context.Context) error {
	s.fs.mu.Lock()
	defer s.fs.mu.Unlock()
	if s.fs.cfg.ReadOnly {
		return store.ErrReadOnly
	}
	s.fs.data.Folders = nil
	s.fs.markDirty()
	return nil
}

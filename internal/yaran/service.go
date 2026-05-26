package yaran

import "fmt"

type Service struct {
	store    *Store
	registry *Registry
}

func NewService(dbPath string) (*Service, error) {
	store, err := NewStore(dbPath)
	if err != nil {
		return nil, err
	}

	return &Service{
		store:    store,
		registry: NewRegistry(),
	}, nil
}

func (s *Service) InitializeDatabase() error {
	return s.store.Init()
}

func (s *Service) ListAddresses(filters Filters) ([]Address, error) {
	return s.store.List(filters)
}

func (s *Service) GetAddress(id int64) (Address, error) {
	return s.store.Get(id)
}

func (s *Service) AddAddress(address Address) (Address, error) {
	return s.store.Insert(address)
}

func (s *Service) UpdateAddress(id int64, address Address) (Address, error) {
	return s.store.Update(id, address)
}

func (s *Service) DeleteAddress(id int64) error {
	return s.store.Delete(id)
}

func (s *Service) ImportAddresses(path string, fileFormat string, progress ProgressFunc) error {
	handler, err := s.registry.Get(fileFormat)
	if err != nil {
		return err
	}
	return handler.Import(path, s.store, progress)
}

func (s *Service) ExportAddresses(path string, fileFormat string, filters Filters, progress ProgressFunc) error {
	handler, err := s.registry.Get(fileFormat)
	if err != nil {
		return err
	}

	addresses, err := s.ListAddresses(filters)
	if err != nil {
		return err
	}

	if err := handler.Export(path, addresses, progress); err != nil {
		return err
	}
	return nil
}

func (s *Service) AvailableFormats() []string {
	return s.registry.AvailableFormats()
}

func (s *Service) Close() error {
	if s.store == nil {
		return nil
	}
	if err := s.store.Close(); err != nil {
		return fmt.Errorf("close store: %w", err)
	}
	return nil
}

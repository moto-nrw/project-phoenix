package userpass

import (
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/mock"
)

// MockAuthStore is a mock implementation of the AuthStorer interface
type MockAuthStore struct {
	mock.Mock
}

func (m *MockAuthStore) GetAccount(id int) (*Account, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Account), args.Error(1)
}

func (m *MockAuthStore) GetAccountByEmail(email string) (*Account, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Account), args.Error(1)
}

func (m *MockAuthStore) CreateAccount(a *Account) error {
	args := m.Called(a)
	return args.Error(0)
}

func (m *MockAuthStore) UpdateAccount(a *Account) error {
	args := m.Called(a)
	return args.Error(0)
}

func (m *MockAuthStore) UpdateAccountPassword(id int, passwordHash string) error {
	args := m.Called(id, passwordHash)
	return args.Error(0)
}

func (m *MockAuthStore) GetToken(token string) (*jwt.Token, error) {
	args := m.Called(token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.Token), args.Error(1)
}

func (m *MockAuthStore) CreateOrUpdateToken(t *jwt.Token) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockAuthStore) DeleteToken(t *jwt.Token) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *MockAuthStore) PurgeExpiredToken() error {
	args := m.Called()
	return args.Error(0)
}
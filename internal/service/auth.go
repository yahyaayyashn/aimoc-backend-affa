package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"aimoc-backend/internal/domain"
	jwtpkg "aimoc-backend/pkg/jwt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	DB  *gorm.DB
	JWT *jwtpkg.Service
}

func NewAuthService(db *gorm.DB, j *jwtpkg.Service) *AuthService { return &AuthService{DB: db, JWT: j} }

type LoginResult struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresAt    time.Time    `json:"expires_at"`
	User         *domain.User `json:"user"`
}

func sha256hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func (s *AuthService) Login(email, password, ip, ua string) (*LoginResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var u domain.User
	if err := s.DB.Preload("Role").Where("LOWER(email) = ?", email).First(&u).Error; err != nil {
		return nil, errors.New("Email atau kata sandi salah")
	}
	if u.Status != "AKTIF" {
		return nil, errors.New("Akun tidak aktif, hubungi admin")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("Email atau kata sandi salah")
	}
	role := ""
	if u.Role != nil {
		role = u.Role.Code
	}
	at, err := s.JWT.GenerateAccess(u.ID, role, u.Email)
	if err != nil {
		return nil, err
	}
	rt, exp, err := s.JWT.GenerateRefresh(u.ID)
	if err != nil {
		return nil, err
	}
	tok := domain.RefreshToken{
		UserID:    u.ID,
		TokenHash: sha256hex(rt),
		ExpiresAt: exp,
		IP:        ip,
		UserAgent: ua,
	}
	s.DB.Create(&tok)
	now := time.Now()
	s.DB.Model(&domain.User{}).Where("id = ?", u.ID).Update("last_login_at", &now)
	return &LoginResult{AccessToken: at, RefreshToken: rt, ExpiresAt: exp, User: &u}, nil
}

func (s *AuthService) RegisterCustomer(req RegisterCustomerReq) (*domain.User, error) {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	var exist int64
	s.DB.Model(&domain.User{}).Where("LOWER(email) = ?", req.Email).Count(&exist)
	if exist > 0 {
		return nil, errors.New("Email sudah terdaftar")
	}
	var role domain.Role
	if err := s.DB.Where("code = ?", "CUSTOMER").First(&role).Error; err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	cust := domain.Customer{
		Code:   "CUS-" + strings.ToUpper(uuid.NewString()[:8]),
		Name:   req.FullName,
		Type:   "INDIVIDU",
		Phone:  req.Phone,
		Email:  req.Email,
		Status: "AKTIF",
	}
	if err := s.DB.Create(&cust).Error; err != nil {
		return nil, err
	}
	u := domain.User{
		RoleID:       role.ID,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hash),
		FullName:     req.FullName,
		Status:       "AKTIF",
		CustomerID:   &cust.ID,
	}
	if err := s.DB.Create(&u).Error; err != nil {
		return nil, err
	}
	u.Role = &role
	return &u, nil
}

type RegisterCustomerReq struct {
	FullName string `json:"full_name" validate:"required,min=3"`
	Email    string `json:"email" validate:"required,email"`
	Phone    string `json:"phone" validate:"required,min=8"`
	Password string `json:"password" validate:"required,min=6"`
}

func (s *AuthService) Refresh(refreshToken, ip, ua string) (*LoginResult, error) {
	claims, err := s.JWT.ParseRefresh(refreshToken)
	if err != nil {
		return nil, err
	}
	hash := sha256hex(refreshToken)
	var tok domain.RefreshToken
	if err := s.DB.Where("token_hash = ? AND revoked = false", hash).First(&tok).Error; err != nil {
		return nil, errors.New("Refresh token tidak ditemukan")
	}
	if time.Now().After(tok.ExpiresAt) {
		return nil, errors.New("Refresh token kadaluarsa")
	}
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, err
	}
	var u domain.User
	if err := s.DB.Preload("Role").First(&u, "id = ?", uid).Error; err != nil {
		return nil, err
	}
	role := ""
	if u.Role != nil {
		role = u.Role.Code
	}
	at, err := s.JWT.GenerateAccess(u.ID, role, u.Email)
	if err != nil {
		return nil, err
	}
	rt, exp, err := s.JWT.GenerateRefresh(u.ID)
	if err != nil {
		return nil, err
	}
	s.DB.Model(&tok).Update("revoked", true)
	s.DB.Create(&domain.RefreshToken{
		UserID: u.ID, TokenHash: sha256hex(rt), ExpiresAt: exp, IP: ip, UserAgent: ua,
	})
	return &LoginResult{AccessToken: at, RefreshToken: rt, ExpiresAt: exp, User: &u}, nil
}

func (s *AuthService) Logout(refreshToken string) error {
	hash := sha256hex(refreshToken)
	return s.DB.Model(&domain.RefreshToken{}).Where("token_hash = ?", hash).Update("revoked", true).Error
}

func (s *AuthService) Me(id uuid.UUID) (*domain.User, error) {
	var u domain.User
	if err := s.DB.
		Preload("Role").
		Preload("Role.Permissions").
		Preload("Excavator", func(db *gorm.DB) *gorm.DB { return db.Preload("Camera") }).
		First(&u, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateMe -- profil self-service (05 Agu 2026, sebelumnya Profile.tsx 100% read-only,
// satu-satunya jalur edit user adalah PUT /users/:id yang SUPER_ADMIN-only). Field yang
// boleh diubah SENGAJA dibatasi ke yang murni personal (nama/telepon/foto) -- role,
// email, status, excavator_id, is_test_viewer TIDAK bisa lewat sini, harus tetap lewat
// Master User (admin-only), supaya user tidak bisa naikkan privilege/ubah scope sendiri.
func (s *AuthService) UpdateMe(id uuid.UUID, fullName, phone, avatarURL *string) (*domain.User, error) {
	updates := map[string]interface{}{}
	if fullName != nil {
		updates["full_name"] = *fullName
	}
	if phone != nil {
		updates["phone"] = *phone
	}
	if avatarURL != nil {
		updates["avatar_url"] = *avatarURL
	}
	if len(updates) > 0 {
		if err := s.DB.Model(&domain.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return s.Me(id)
}

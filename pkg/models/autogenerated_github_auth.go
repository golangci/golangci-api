// Code generated by go-queryset. DO NOT EDIT.
package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
)

// ===== BEGIN of all query sets

// ===== BEGIN of query set GithubAuthQuerySet

// GithubAuthQuerySet is an queryset type for GithubAuth
type GithubAuthQuerySet struct {
	db *gorm.DB
}

// NewGithubAuthQuerySet constructs new GithubAuthQuerySet
func NewGithubAuthQuerySet(db *gorm.DB) GithubAuthQuerySet {
	return GithubAuthQuerySet{
		db: db.Model(&GithubAuth{}),
	}
}

func (qs GithubAuthQuerySet) w(db *gorm.DB) GithubAuthQuerySet {
	return NewGithubAuthQuerySet(db)
}

// AccessTokenEq is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) AccessTokenEq(accessToken string) GithubAuthQuerySet {
	return qs.w(qs.db.Where("access_token = ?", accessToken))
}

// AccessTokenIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) AccessTokenIn(accessToken ...string) GithubAuthQuerySet {
	if len(accessToken) == 0 {
		qs.db.AddError(errors.New("must at least pass one accessToken in AccessTokenIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("access_token IN (?)", accessToken))
}

// AccessTokenNe is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) AccessTokenNe(accessToken string) GithubAuthQuerySet {
	return qs.w(qs.db.Where("access_token != ?", accessToken))
}

// AccessTokenNotIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) AccessTokenNotIn(accessToken ...string) GithubAuthQuerySet {
	if len(accessToken) == 0 {
		qs.db.AddError(errors.New("must at least pass one accessToken in AccessTokenNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("access_token NOT IN (?)", accessToken))
}

// All is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) All(ret *[]GithubAuth) error {
	return qs.db.Find(ret).Error
}

// Count is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) Count() (int, error) {
	var count int
	err := qs.db.Count(&count).Error
	return count, err
}

// Create is an autogenerated method
// nolint: dupl
func (o *GithubAuth) Create(db *gorm.DB) error {
	return db.Create(o).Error
}

// CreatedAtEq is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) CreatedAtEq(createdAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("created_at = ?", createdAt))
}

// CreatedAtGt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) CreatedAtGt(createdAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("created_at > ?", createdAt))
}

// CreatedAtGte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) CreatedAtGte(createdAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("created_at >= ?", createdAt))
}

// CreatedAtLt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) CreatedAtLt(createdAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("created_at < ?", createdAt))
}

// CreatedAtLte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) CreatedAtLte(createdAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("created_at <= ?", createdAt))
}

// CreatedAtNe is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) CreatedAtNe(createdAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("created_at != ?", createdAt))
}

// Delete is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) Delete() error {
	return qs.db.Delete(GithubAuth{}).Error
}

// Delete is an autogenerated method
// nolint: dupl
func (o *GithubAuth) Delete(db *gorm.DB) error {
	return db.Delete(o).Error
}

// DeletedAtEq is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) DeletedAtEq(deletedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("deleted_at = ?", deletedAt))
}

// DeletedAtGt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) DeletedAtGt(deletedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("deleted_at > ?", deletedAt))
}

// DeletedAtGte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) DeletedAtGte(deletedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("deleted_at >= ?", deletedAt))
}

// DeletedAtIsNotNull is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) DeletedAtIsNotNull() GithubAuthQuerySet {
	return qs.w(qs.db.Where("deleted_at IS NOT NULL"))
}

// DeletedAtIsNull is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) DeletedAtIsNull() GithubAuthQuerySet {
	return qs.w(qs.db.Where("deleted_at IS NULL"))
}

// DeletedAtLt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) DeletedAtLt(deletedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("deleted_at < ?", deletedAt))
}

// DeletedAtLte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) DeletedAtLte(deletedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("deleted_at <= ?", deletedAt))
}

// DeletedAtNe is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) DeletedAtNe(deletedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("deleted_at != ?", deletedAt))
}

// GetUpdater is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) GetUpdater() GithubAuthUpdater {
	return NewGithubAuthUpdater(qs.db)
}

// GithubUserIDEq is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) GithubUserIDEq(githubUserID uint64) GithubAuthQuerySet {
	return qs.w(qs.db.Where("github_user_id = ?", githubUserID))
}

// GithubUserIDGt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) GithubUserIDGt(githubUserID uint64) GithubAuthQuerySet {
	return qs.w(qs.db.Where("github_user_id > ?", githubUserID))
}

// GithubUserIDGte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) GithubUserIDGte(githubUserID uint64) GithubAuthQuerySet {
	return qs.w(qs.db.Where("github_user_id >= ?", githubUserID))
}

// GithubUserIDIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) GithubUserIDIn(githubUserID ...uint64) GithubAuthQuerySet {
	if len(githubUserID) == 0 {
		qs.db.AddError(errors.New("must at least pass one githubUserID in GithubUserIDIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("github_user_id IN (?)", githubUserID))
}

// GithubUserIDLt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) GithubUserIDLt(githubUserID uint64) GithubAuthQuerySet {
	return qs.w(qs.db.Where("github_user_id < ?", githubUserID))
}

// GithubUserIDLte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) GithubUserIDLte(githubUserID uint64) GithubAuthQuerySet {
	return qs.w(qs.db.Where("github_user_id <= ?", githubUserID))
}

// GithubUserIDNe is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) GithubUserIDNe(githubUserID uint64) GithubAuthQuerySet {
	return qs.w(qs.db.Where("github_user_id != ?", githubUserID))
}

// GithubUserIDNotIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) GithubUserIDNotIn(githubUserID ...uint64) GithubAuthQuerySet {
	if len(githubUserID) == 0 {
		qs.db.AddError(errors.New("must at least pass one githubUserID in GithubUserIDNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("github_user_id NOT IN (?)", githubUserID))
}

// IDEq is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) IDEq(ID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("id = ?", ID))
}

// IDGt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) IDGt(ID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("id > ?", ID))
}

// IDGte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) IDGte(ID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("id >= ?", ID))
}

// IDIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) IDIn(ID ...uint) GithubAuthQuerySet {
	if len(ID) == 0 {
		qs.db.AddError(errors.New("must at least pass one ID in IDIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("id IN (?)", ID))
}

// IDLt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) IDLt(ID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("id < ?", ID))
}

// IDLte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) IDLte(ID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("id <= ?", ID))
}

// IDNe is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) IDNe(ID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("id != ?", ID))
}

// IDNotIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) IDNotIn(ID ...uint) GithubAuthQuerySet {
	if len(ID) == 0 {
		qs.db.AddError(errors.New("must at least pass one ID in IDNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("id NOT IN (?)", ID))
}

// Limit is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) Limit(limit int) GithubAuthQuerySet {
	return qs.w(qs.db.Limit(limit))
}

// LoginEq is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) LoginEq(login string) GithubAuthQuerySet {
	return qs.w(qs.db.Where("login = ?", login))
}

// LoginIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) LoginIn(login ...string) GithubAuthQuerySet {
	if len(login) == 0 {
		qs.db.AddError(errors.New("must at least pass one login in LoginIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("login IN (?)", login))
}

// LoginNe is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) LoginNe(login string) GithubAuthQuerySet {
	return qs.w(qs.db.Where("login != ?", login))
}

// LoginNotIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) LoginNotIn(login ...string) GithubAuthQuerySet {
	if len(login) == 0 {
		qs.db.AddError(errors.New("must at least pass one login in LoginNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("login NOT IN (?)", login))
}

// Offset is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) Offset(offset int) GithubAuthQuerySet {
	return qs.w(qs.db.Offset(offset))
}

// One is used to retrieve one result. It returns gorm.ErrRecordNotFound
// if nothing was fetched
func (qs GithubAuthQuerySet) One(ret *GithubAuth) error {
	return qs.db.First(ret).Error
}

// OrderAscByCreatedAt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderAscByCreatedAt() GithubAuthQuerySet {
	return qs.w(qs.db.Order("created_at ASC"))
}

// OrderAscByDeletedAt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderAscByDeletedAt() GithubAuthQuerySet {
	return qs.w(qs.db.Order("deleted_at ASC"))
}

// OrderAscByGithubUserID is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderAscByGithubUserID() GithubAuthQuerySet {
	return qs.w(qs.db.Order("github_user_id ASC"))
}

// OrderAscByID is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderAscByID() GithubAuthQuerySet {
	return qs.w(qs.db.Order("id ASC"))
}

// OrderAscByUpdatedAt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderAscByUpdatedAt() GithubAuthQuerySet {
	return qs.w(qs.db.Order("updated_at ASC"))
}

// OrderAscByUserID is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderAscByUserID() GithubAuthQuerySet {
	return qs.w(qs.db.Order("user_id ASC"))
}

// OrderDescByCreatedAt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderDescByCreatedAt() GithubAuthQuerySet {
	return qs.w(qs.db.Order("created_at DESC"))
}

// OrderDescByDeletedAt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderDescByDeletedAt() GithubAuthQuerySet {
	return qs.w(qs.db.Order("deleted_at DESC"))
}

// OrderDescByGithubUserID is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderDescByGithubUserID() GithubAuthQuerySet {
	return qs.w(qs.db.Order("github_user_id DESC"))
}

// OrderDescByID is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderDescByID() GithubAuthQuerySet {
	return qs.w(qs.db.Order("id DESC"))
}

// OrderDescByUpdatedAt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderDescByUpdatedAt() GithubAuthQuerySet {
	return qs.w(qs.db.Order("updated_at DESC"))
}

// OrderDescByUserID is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) OrderDescByUserID() GithubAuthQuerySet {
	return qs.w(qs.db.Order("user_id DESC"))
}

// PrivateAccessTokenEq is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) PrivateAccessTokenEq(privateAccessToken string) GithubAuthQuerySet {
	return qs.w(qs.db.Where("private_access_token = ?", privateAccessToken))
}

// PrivateAccessTokenIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) PrivateAccessTokenIn(privateAccessToken ...string) GithubAuthQuerySet {
	if len(privateAccessToken) == 0 {
		qs.db.AddError(errors.New("must at least pass one privateAccessToken in PrivateAccessTokenIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("private_access_token IN (?)", privateAccessToken))
}

// PrivateAccessTokenNe is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) PrivateAccessTokenNe(privateAccessToken string) GithubAuthQuerySet {
	return qs.w(qs.db.Where("private_access_token != ?", privateAccessToken))
}

// PrivateAccessTokenNotIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) PrivateAccessTokenNotIn(privateAccessToken ...string) GithubAuthQuerySet {
	if len(privateAccessToken) == 0 {
		qs.db.AddError(errors.New("must at least pass one privateAccessToken in PrivateAccessTokenNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("private_access_token NOT IN (?)", privateAccessToken))
}

// SetAccessToken is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) SetAccessToken(accessToken string) GithubAuthUpdater {
	u.fields[string(GithubAuthDBSchema.AccessToken)] = accessToken
	return u
}

// SetCreatedAt is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) SetCreatedAt(createdAt time.Time) GithubAuthUpdater {
	u.fields[string(GithubAuthDBSchema.CreatedAt)] = createdAt
	return u
}

// SetDeletedAt is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) SetDeletedAt(deletedAt *time.Time) GithubAuthUpdater {
	u.fields[string(GithubAuthDBSchema.DeletedAt)] = deletedAt
	return u
}

// SetGithubUserID is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) SetGithubUserID(githubUserID uint64) GithubAuthUpdater {
	u.fields[string(GithubAuthDBSchema.GithubUserID)] = githubUserID
	return u
}

// SetID is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) SetID(ID uint) GithubAuthUpdater {
	u.fields[string(GithubAuthDBSchema.ID)] = ID
	return u
}

// SetLogin is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) SetLogin(login string) GithubAuthUpdater {
	u.fields[string(GithubAuthDBSchema.Login)] = login
	return u
}

// SetPrivateAccessToken is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) SetPrivateAccessToken(privateAccessToken string) GithubAuthUpdater {
	u.fields[string(GithubAuthDBSchema.PrivateAccessToken)] = privateAccessToken
	return u
}

// SetUpdatedAt is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) SetUpdatedAt(updatedAt time.Time) GithubAuthUpdater {
	u.fields[string(GithubAuthDBSchema.UpdatedAt)] = updatedAt
	return u
}

// SetUserID is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) SetUserID(userID uint) GithubAuthUpdater {
	u.fields[string(GithubAuthDBSchema.UserID)] = userID
	return u
}

// Update is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) Update() error {
	return u.db.Updates(u.fields).Error
}

// UpdateNum is an autogenerated method
// nolint: dupl
func (u GithubAuthUpdater) UpdateNum() (int64, error) {
	db := u.db.Updates(u.fields)
	return db.RowsAffected, db.Error
}

// UpdatedAtEq is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UpdatedAtEq(updatedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("updated_at = ?", updatedAt))
}

// UpdatedAtGt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UpdatedAtGt(updatedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("updated_at > ?", updatedAt))
}

// UpdatedAtGte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UpdatedAtGte(updatedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("updated_at >= ?", updatedAt))
}

// UpdatedAtLt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UpdatedAtLt(updatedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("updated_at < ?", updatedAt))
}

// UpdatedAtLte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UpdatedAtLte(updatedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("updated_at <= ?", updatedAt))
}

// UpdatedAtNe is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UpdatedAtNe(updatedAt time.Time) GithubAuthQuerySet {
	return qs.w(qs.db.Where("updated_at != ?", updatedAt))
}

// UserIDEq is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UserIDEq(userID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("user_id = ?", userID))
}

// UserIDGt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UserIDGt(userID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("user_id > ?", userID))
}

// UserIDGte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UserIDGte(userID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("user_id >= ?", userID))
}

// UserIDIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UserIDIn(userID ...uint) GithubAuthQuerySet {
	if len(userID) == 0 {
		qs.db.AddError(errors.New("must at least pass one userID in UserIDIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("user_id IN (?)", userID))
}

// UserIDLt is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UserIDLt(userID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("user_id < ?", userID))
}

// UserIDLte is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UserIDLte(userID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("user_id <= ?", userID))
}

// UserIDNe is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UserIDNe(userID uint) GithubAuthQuerySet {
	return qs.w(qs.db.Where("user_id != ?", userID))
}

// UserIDNotIn is an autogenerated method
// nolint: dupl
func (qs GithubAuthQuerySet) UserIDNotIn(userID ...uint) GithubAuthQuerySet {
	if len(userID) == 0 {
		qs.db.AddError(errors.New("must at least pass one userID in UserIDNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("user_id NOT IN (?)", userID))
}

// ===== END of query set GithubAuthQuerySet

// ===== BEGIN of GithubAuth modifiers

// GithubAuthDBSchemaField describes database schema field. It requires for method 'Update'
type GithubAuthDBSchemaField string

// String method returns string representation of field.
// nolint: dupl
func (f GithubAuthDBSchemaField) String() string {
	return string(f)
}

// GithubAuthDBSchema stores db field names of GithubAuth
var GithubAuthDBSchema = struct {
	ID                 GithubAuthDBSchemaField
	CreatedAt          GithubAuthDBSchemaField
	UpdatedAt          GithubAuthDBSchemaField
	DeletedAt          GithubAuthDBSchemaField
	AccessToken        GithubAuthDBSchemaField
	PrivateAccessToken GithubAuthDBSchemaField
	UserID             GithubAuthDBSchemaField
	GithubUserID       GithubAuthDBSchemaField
	Login              GithubAuthDBSchemaField
}{

	ID:                 GithubAuthDBSchemaField("id"),
	CreatedAt:          GithubAuthDBSchemaField("created_at"),
	UpdatedAt:          GithubAuthDBSchemaField("updated_at"),
	DeletedAt:          GithubAuthDBSchemaField("deleted_at"),
	AccessToken:        GithubAuthDBSchemaField("access_token"),
	PrivateAccessToken: GithubAuthDBSchemaField("private_access_token"),
	UserID:             GithubAuthDBSchemaField("user_id"),
	GithubUserID:       GithubAuthDBSchemaField("github_user_id"),
	Login:              GithubAuthDBSchemaField("login"),
}

// Update updates GithubAuth fields by primary key
// nolint: dupl
func (o *GithubAuth) Update(db *gorm.DB, fields ...GithubAuthDBSchemaField) error {
	dbNameToFieldName := map[string]interface{}{
		"id":                   o.ID,
		"created_at":           o.CreatedAt,
		"updated_at":           o.UpdatedAt,
		"deleted_at":           o.DeletedAt,
		"access_token":         o.AccessToken,
		"private_access_token": o.PrivateAccessToken,
		"user_id":              o.UserID,
		"github_user_id":       o.GithubUserID,
		"login":                o.Login,
	}
	u := map[string]interface{}{}
	for _, f := range fields {
		fs := f.String()
		u[fs] = dbNameToFieldName[fs]
	}
	if err := db.Model(o).Updates(u).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return err
		}

		return fmt.Errorf("can't update GithubAuth %v fields %v: %s",
			o, fields, err)
	}

	return nil
}

// GithubAuthUpdater is an GithubAuth updates manager
type GithubAuthUpdater struct {
	fields map[string]interface{}
	db     *gorm.DB
}

// NewGithubAuthUpdater creates new GithubAuth updater
// nolint: dupl
func NewGithubAuthUpdater(db *gorm.DB) GithubAuthUpdater {
	return GithubAuthUpdater{
		fields: map[string]interface{}{},
		db:     db.Model(&GithubAuth{}),
	}
}

// ===== END of GithubAuth modifiers

// ===== END of all query sets
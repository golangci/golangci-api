// Code generated by go-queryset. DO NOT EDIT.
package models

import (
	"errors"
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
)

// ===== BEGIN of all query sets

// ===== BEGIN of query set RepoQuerySet

// RepoQuerySet is an queryset type for Repo
type RepoQuerySet struct {
	db *gorm.DB
}

// NewRepoQuerySet constructs new RepoQuerySet
func NewRepoQuerySet(db *gorm.DB) RepoQuerySet {
	return RepoQuerySet{
		db: db.Model(&Repo{}),
	}
}

func (qs RepoQuerySet) w(db *gorm.DB) RepoQuerySet {
	return NewRepoQuerySet(db)
}

// All is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) All(ret *[]Repo) error {
	return qs.db.Find(ret).Error
}

// CommitStateEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CommitStateEq(commitState RepoCommitState) RepoQuerySet {
	return qs.w(qs.db.Where("commit_state = ?", commitState))
}

// CommitStateIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CommitStateIn(commitState ...RepoCommitState) RepoQuerySet {
	if len(commitState) == 0 {
		qs.db.AddError(errors.New("must at least pass one commitState in CommitStateIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("commit_state IN (?)", commitState))
}

// CommitStateNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CommitStateNe(commitState RepoCommitState) RepoQuerySet {
	return qs.w(qs.db.Where("commit_state != ?", commitState))
}

// CommitStateNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CommitStateNotIn(commitState ...RepoCommitState) RepoQuerySet {
	if len(commitState) == 0 {
		qs.db.AddError(errors.New("must at least pass one commitState in CommitStateNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("commit_state NOT IN (?)", commitState))
}

// Count is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) Count() (int, error) {
	var count int
	err := qs.db.Count(&count).Error
	return count, err
}

// Create is an autogenerated method
// nolint: dupl
func (o *Repo) Create(db *gorm.DB) error {
	return db.Create(o).Error
}

// CreateFailReasonEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CreateFailReasonEq(createFailReason string) RepoQuerySet {
	return qs.w(qs.db.Where("create_fail_reason = ?", createFailReason))
}

// CreateFailReasonIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CreateFailReasonIn(createFailReason ...string) RepoQuerySet {
	if len(createFailReason) == 0 {
		qs.db.AddError(errors.New("must at least pass one createFailReason in CreateFailReasonIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("create_fail_reason IN (?)", createFailReason))
}

// CreateFailReasonNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CreateFailReasonNe(createFailReason string) RepoQuerySet {
	return qs.w(qs.db.Where("create_fail_reason != ?", createFailReason))
}

// CreateFailReasonNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CreateFailReasonNotIn(createFailReason ...string) RepoQuerySet {
	if len(createFailReason) == 0 {
		qs.db.AddError(errors.New("must at least pass one createFailReason in CreateFailReasonNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("create_fail_reason NOT IN (?)", createFailReason))
}

// CreatedAtEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CreatedAtEq(createdAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("created_at = ?", createdAt))
}

// CreatedAtGt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CreatedAtGt(createdAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("created_at > ?", createdAt))
}

// CreatedAtGte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CreatedAtGte(createdAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("created_at >= ?", createdAt))
}

// CreatedAtLt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CreatedAtLt(createdAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("created_at < ?", createdAt))
}

// CreatedAtLte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CreatedAtLte(createdAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("created_at <= ?", createdAt))
}

// CreatedAtNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) CreatedAtNe(createdAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("created_at != ?", createdAt))
}

// Delete is an autogenerated method
// nolint: dupl
func (o *Repo) Delete(db *gorm.DB) error {
	return db.Delete(o).Error
}

// Delete is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) Delete() error {
	return qs.db.Delete(Repo{}).Error
}

// DeleteNum is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DeleteNum() (int64, error) {
	db := qs.db.Delete(Repo{})
	return db.RowsAffected, db.Error
}

// DeleteNumUnscoped is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DeleteNumUnscoped() (int64, error) {
	db := qs.db.Unscoped().Delete(Repo{})
	return db.RowsAffected, db.Error
}

// DeletedAtEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DeletedAtEq(deletedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("deleted_at = ?", deletedAt))
}

// DeletedAtGt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DeletedAtGt(deletedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("deleted_at > ?", deletedAt))
}

// DeletedAtGte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DeletedAtGte(deletedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("deleted_at >= ?", deletedAt))
}

// DeletedAtIsNotNull is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DeletedAtIsNotNull() RepoQuerySet {
	return qs.w(qs.db.Where("deleted_at IS NOT NULL"))
}

// DeletedAtIsNull is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DeletedAtIsNull() RepoQuerySet {
	return qs.w(qs.db.Where("deleted_at IS NULL"))
}

// DeletedAtLt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DeletedAtLt(deletedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("deleted_at < ?", deletedAt))
}

// DeletedAtLte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DeletedAtLte(deletedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("deleted_at <= ?", deletedAt))
}

// DeletedAtNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DeletedAtNe(deletedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("deleted_at != ?", deletedAt))
}

// DisplayFullNameEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DisplayFullNameEq(displayFullName string) RepoQuerySet {
	return qs.w(qs.db.Where("display_name = ?", displayFullName))
}

// DisplayFullNameIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DisplayFullNameIn(displayFullName ...string) RepoQuerySet {
	if len(displayFullName) == 0 {
		qs.db.AddError(errors.New("must at least pass one displayFullName in DisplayFullNameIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("display_name IN (?)", displayFullName))
}

// DisplayFullNameNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DisplayFullNameNe(displayFullName string) RepoQuerySet {
	return qs.w(qs.db.Where("display_name != ?", displayFullName))
}

// DisplayFullNameNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) DisplayFullNameNotIn(displayFullName ...string) RepoQuerySet {
	if len(displayFullName) == 0 {
		qs.db.AddError(errors.New("must at least pass one displayFullName in DisplayFullNameNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("display_name NOT IN (?)", displayFullName))
}

// FullNameEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) FullNameEq(fullName string) RepoQuerySet {
	return qs.w(qs.db.Where("name = ?", fullName))
}

// FullNameIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) FullNameIn(fullName ...string) RepoQuerySet {
	if len(fullName) == 0 {
		qs.db.AddError(errors.New("must at least pass one fullName in FullNameIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("name IN (?)", fullName))
}

// FullNameNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) FullNameNe(fullName string) RepoQuerySet {
	return qs.w(qs.db.Where("name != ?", fullName))
}

// FullNameNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) FullNameNotIn(fullName ...string) RepoQuerySet {
	if len(fullName) == 0 {
		qs.db.AddError(errors.New("must at least pass one fullName in FullNameNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("name NOT IN (?)", fullName))
}

// GetUpdater is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) GetUpdater() RepoUpdater {
	return NewRepoUpdater(qs.db)
}

// HookIDEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) HookIDEq(hookID string) RepoQuerySet {
	return qs.w(qs.db.Where("hook_id = ?", hookID))
}

// HookIDIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) HookIDIn(hookID ...string) RepoQuerySet {
	if len(hookID) == 0 {
		qs.db.AddError(errors.New("must at least pass one hookID in HookIDIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("hook_id IN (?)", hookID))
}

// HookIDNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) HookIDNe(hookID string) RepoQuerySet {
	return qs.w(qs.db.Where("hook_id != ?", hookID))
}

// HookIDNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) HookIDNotIn(hookID ...string) RepoQuerySet {
	if len(hookID) == 0 {
		qs.db.AddError(errors.New("must at least pass one hookID in HookIDNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("hook_id NOT IN (?)", hookID))
}

// IDEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IDEq(ID uint) RepoQuerySet {
	return qs.w(qs.db.Where("id = ?", ID))
}

// IDGt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IDGt(ID uint) RepoQuerySet {
	return qs.w(qs.db.Where("id > ?", ID))
}

// IDGte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IDGte(ID uint) RepoQuerySet {
	return qs.w(qs.db.Where("id >= ?", ID))
}

// IDIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IDIn(ID ...uint) RepoQuerySet {
	if len(ID) == 0 {
		qs.db.AddError(errors.New("must at least pass one ID in IDIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("id IN (?)", ID))
}

// IDLt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IDLt(ID uint) RepoQuerySet {
	return qs.w(qs.db.Where("id < ?", ID))
}

// IDLte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IDLte(ID uint) RepoQuerySet {
	return qs.w(qs.db.Where("id <= ?", ID))
}

// IDNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IDNe(ID uint) RepoQuerySet {
	return qs.w(qs.db.Where("id != ?", ID))
}

// IDNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IDNotIn(ID ...uint) RepoQuerySet {
	if len(ID) == 0 {
		qs.db.AddError(errors.New("must at least pass one ID in IDNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("id NOT IN (?)", ID))
}

// IsPrivateEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IsPrivateEq(isPrivate bool) RepoQuerySet {
	return qs.w(qs.db.Where("is_private = ?", isPrivate))
}

// IsPrivateIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IsPrivateIn(isPrivate ...bool) RepoQuerySet {
	if len(isPrivate) == 0 {
		qs.db.AddError(errors.New("must at least pass one isPrivate in IsPrivateIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("is_private IN (?)", isPrivate))
}

// IsPrivateNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IsPrivateNe(isPrivate bool) RepoQuerySet {
	return qs.w(qs.db.Where("is_private != ?", isPrivate))
}

// IsPrivateNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) IsPrivateNotIn(isPrivate ...bool) RepoQuerySet {
	if len(isPrivate) == 0 {
		qs.db.AddError(errors.New("must at least pass one isPrivate in IsPrivateNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("is_private NOT IN (?)", isPrivate))
}

// Limit is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) Limit(limit int) RepoQuerySet {
	return qs.w(qs.db.Limit(limit))
}

// Offset is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) Offset(offset int) RepoQuerySet {
	return qs.w(qs.db.Offset(offset))
}

// One is used to retrieve one result. It returns gorm.ErrRecordNotFound
// if nothing was fetched
func (qs RepoQuerySet) One(ret *Repo) error {
	return qs.db.First(ret).Error
}

// OrderAscByCreatedAt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderAscByCreatedAt() RepoQuerySet {
	return qs.w(qs.db.Order("created_at ASC"))
}

// OrderAscByDeletedAt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderAscByDeletedAt() RepoQuerySet {
	return qs.w(qs.db.Order("deleted_at ASC"))
}

// OrderAscByID is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderAscByID() RepoQuerySet {
	return qs.w(qs.db.Order("id ASC"))
}

// OrderAscByProviderHookID is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderAscByProviderHookID() RepoQuerySet {
	return qs.w(qs.db.Order("provider_hook_id ASC"))
}

// OrderAscByProviderID is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderAscByProviderID() RepoQuerySet {
	return qs.w(qs.db.Order("provider_id ASC"))
}

// OrderAscByStargazersCount is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderAscByStargazersCount() RepoQuerySet {
	return qs.w(qs.db.Order("stargazers_count ASC"))
}

// OrderAscByUpdatedAt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderAscByUpdatedAt() RepoQuerySet {
	return qs.w(qs.db.Order("updated_at ASC"))
}

// OrderAscByUserID is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderAscByUserID() RepoQuerySet {
	return qs.w(qs.db.Order("user_id ASC"))
}

// OrderDescByCreatedAt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderDescByCreatedAt() RepoQuerySet {
	return qs.w(qs.db.Order("created_at DESC"))
}

// OrderDescByDeletedAt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderDescByDeletedAt() RepoQuerySet {
	return qs.w(qs.db.Order("deleted_at DESC"))
}

// OrderDescByID is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderDescByID() RepoQuerySet {
	return qs.w(qs.db.Order("id DESC"))
}

// OrderDescByProviderHookID is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderDescByProviderHookID() RepoQuerySet {
	return qs.w(qs.db.Order("provider_hook_id DESC"))
}

// OrderDescByProviderID is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderDescByProviderID() RepoQuerySet {
	return qs.w(qs.db.Order("provider_id DESC"))
}

// OrderDescByStargazersCount is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderDescByStargazersCount() RepoQuerySet {
	return qs.w(qs.db.Order("stargazers_count DESC"))
}

// OrderDescByUpdatedAt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderDescByUpdatedAt() RepoQuerySet {
	return qs.w(qs.db.Order("updated_at DESC"))
}

// OrderDescByUserID is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) OrderDescByUserID() RepoQuerySet {
	return qs.w(qs.db.Order("user_id DESC"))
}

// ProviderEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderEq(provider string) RepoQuerySet {
	return qs.w(qs.db.Where("provider = ?", provider))
}

// ProviderHookIDEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderHookIDEq(providerHookID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_hook_id = ?", providerHookID))
}

// ProviderHookIDGt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderHookIDGt(providerHookID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_hook_id > ?", providerHookID))
}

// ProviderHookIDGte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderHookIDGte(providerHookID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_hook_id >= ?", providerHookID))
}

// ProviderHookIDIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderHookIDIn(providerHookID ...int) RepoQuerySet {
	if len(providerHookID) == 0 {
		qs.db.AddError(errors.New("must at least pass one providerHookID in ProviderHookIDIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("provider_hook_id IN (?)", providerHookID))
}

// ProviderHookIDLt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderHookIDLt(providerHookID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_hook_id < ?", providerHookID))
}

// ProviderHookIDLte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderHookIDLte(providerHookID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_hook_id <= ?", providerHookID))
}

// ProviderHookIDNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderHookIDNe(providerHookID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_hook_id != ?", providerHookID))
}

// ProviderHookIDNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderHookIDNotIn(providerHookID ...int) RepoQuerySet {
	if len(providerHookID) == 0 {
		qs.db.AddError(errors.New("must at least pass one providerHookID in ProviderHookIDNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("provider_hook_id NOT IN (?)", providerHookID))
}

// ProviderIDEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderIDEq(providerID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_id = ?", providerID))
}

// ProviderIDGt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderIDGt(providerID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_id > ?", providerID))
}

// ProviderIDGte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderIDGte(providerID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_id >= ?", providerID))
}

// ProviderIDIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderIDIn(providerID ...int) RepoQuerySet {
	if len(providerID) == 0 {
		qs.db.AddError(errors.New("must at least pass one providerID in ProviderIDIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("provider_id IN (?)", providerID))
}

// ProviderIDLt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderIDLt(providerID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_id < ?", providerID))
}

// ProviderIDLte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderIDLte(providerID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_id <= ?", providerID))
}

// ProviderIDNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderIDNe(providerID int) RepoQuerySet {
	return qs.w(qs.db.Where("provider_id != ?", providerID))
}

// ProviderIDNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderIDNotIn(providerID ...int) RepoQuerySet {
	if len(providerID) == 0 {
		qs.db.AddError(errors.New("must at least pass one providerID in ProviderIDNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("provider_id NOT IN (?)", providerID))
}

// ProviderIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderIn(provider ...string) RepoQuerySet {
	if len(provider) == 0 {
		qs.db.AddError(errors.New("must at least pass one provider in ProviderIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("provider IN (?)", provider))
}

// ProviderNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderNe(provider string) RepoQuerySet {
	return qs.w(qs.db.Where("provider != ?", provider))
}

// ProviderNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) ProviderNotIn(provider ...string) RepoQuerySet {
	if len(provider) == 0 {
		qs.db.AddError(errors.New("must at least pass one provider in ProviderNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("provider NOT IN (?)", provider))
}

// SetCommitState is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetCommitState(commitState RepoCommitState) RepoUpdater {
	u.fields[string(RepoDBSchema.CommitState)] = commitState
	return u
}

// SetCreateFailReason is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetCreateFailReason(createFailReason string) RepoUpdater {
	u.fields[string(RepoDBSchema.CreateFailReason)] = createFailReason
	return u
}

// SetCreatedAt is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetCreatedAt(createdAt time.Time) RepoUpdater {
	u.fields[string(RepoDBSchema.CreatedAt)] = createdAt
	return u
}

// SetDeletedAt is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetDeletedAt(deletedAt *time.Time) RepoUpdater {
	u.fields[string(RepoDBSchema.DeletedAt)] = deletedAt
	return u
}

// SetDisplayFullName is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetDisplayFullName(displayFullName string) RepoUpdater {
	u.fields[string(RepoDBSchema.DisplayFullName)] = displayFullName
	return u
}

// SetFullName is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetFullName(fullName string) RepoUpdater {
	u.fields[string(RepoDBSchema.FullName)] = fullName
	return u
}

// SetHookID is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetHookID(hookID string) RepoUpdater {
	u.fields[string(RepoDBSchema.HookID)] = hookID
	return u
}

// SetID is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetID(ID uint) RepoUpdater {
	u.fields[string(RepoDBSchema.ID)] = ID
	return u
}

// SetIsPrivate is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetIsPrivate(isPrivate bool) RepoUpdater {
	u.fields[string(RepoDBSchema.IsPrivate)] = isPrivate
	return u
}

// SetProvider is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetProvider(provider string) RepoUpdater {
	u.fields[string(RepoDBSchema.Provider)] = provider
	return u
}

// SetProviderHookID is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetProviderHookID(providerHookID int) RepoUpdater {
	u.fields[string(RepoDBSchema.ProviderHookID)] = providerHookID
	return u
}

// SetProviderID is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetProviderID(providerID int) RepoUpdater {
	u.fields[string(RepoDBSchema.ProviderID)] = providerID
	return u
}

// SetStargazersCount is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetStargazersCount(stargazersCount int) RepoUpdater {
	u.fields[string(RepoDBSchema.StargazersCount)] = stargazersCount
	return u
}

// SetUpdatedAt is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetUpdatedAt(updatedAt time.Time) RepoUpdater {
	u.fields[string(RepoDBSchema.UpdatedAt)] = updatedAt
	return u
}

// SetUserID is an autogenerated method
// nolint: dupl
func (u RepoUpdater) SetUserID(userID uint) RepoUpdater {
	u.fields[string(RepoDBSchema.UserID)] = userID
	return u
}

// StargazersCountEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) StargazersCountEq(stargazersCount int) RepoQuerySet {
	return qs.w(qs.db.Where("stargazers_count = ?", stargazersCount))
}

// StargazersCountGt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) StargazersCountGt(stargazersCount int) RepoQuerySet {
	return qs.w(qs.db.Where("stargazers_count > ?", stargazersCount))
}

// StargazersCountGte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) StargazersCountGte(stargazersCount int) RepoQuerySet {
	return qs.w(qs.db.Where("stargazers_count >= ?", stargazersCount))
}

// StargazersCountIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) StargazersCountIn(stargazersCount ...int) RepoQuerySet {
	if len(stargazersCount) == 0 {
		qs.db.AddError(errors.New("must at least pass one stargazersCount in StargazersCountIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("stargazers_count IN (?)", stargazersCount))
}

// StargazersCountLt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) StargazersCountLt(stargazersCount int) RepoQuerySet {
	return qs.w(qs.db.Where("stargazers_count < ?", stargazersCount))
}

// StargazersCountLte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) StargazersCountLte(stargazersCount int) RepoQuerySet {
	return qs.w(qs.db.Where("stargazers_count <= ?", stargazersCount))
}

// StargazersCountNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) StargazersCountNe(stargazersCount int) RepoQuerySet {
	return qs.w(qs.db.Where("stargazers_count != ?", stargazersCount))
}

// StargazersCountNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) StargazersCountNotIn(stargazersCount ...int) RepoQuerySet {
	if len(stargazersCount) == 0 {
		qs.db.AddError(errors.New("must at least pass one stargazersCount in StargazersCountNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("stargazers_count NOT IN (?)", stargazersCount))
}

// Update is an autogenerated method
// nolint: dupl
func (u RepoUpdater) Update() error {
	return u.db.Updates(u.fields).Error
}

// UpdateNum is an autogenerated method
// nolint: dupl
func (u RepoUpdater) UpdateNum() (int64, error) {
	db := u.db.Updates(u.fields)
	return db.RowsAffected, db.Error
}

// UpdatedAtEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UpdatedAtEq(updatedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("updated_at = ?", updatedAt))
}

// UpdatedAtGt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UpdatedAtGt(updatedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("updated_at > ?", updatedAt))
}

// UpdatedAtGte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UpdatedAtGte(updatedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("updated_at >= ?", updatedAt))
}

// UpdatedAtLt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UpdatedAtLt(updatedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("updated_at < ?", updatedAt))
}

// UpdatedAtLte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UpdatedAtLte(updatedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("updated_at <= ?", updatedAt))
}

// UpdatedAtNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UpdatedAtNe(updatedAt time.Time) RepoQuerySet {
	return qs.w(qs.db.Where("updated_at != ?", updatedAt))
}

// UserIDEq is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UserIDEq(userID uint) RepoQuerySet {
	return qs.w(qs.db.Where("user_id = ?", userID))
}

// UserIDGt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UserIDGt(userID uint) RepoQuerySet {
	return qs.w(qs.db.Where("user_id > ?", userID))
}

// UserIDGte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UserIDGte(userID uint) RepoQuerySet {
	return qs.w(qs.db.Where("user_id >= ?", userID))
}

// UserIDIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UserIDIn(userID ...uint) RepoQuerySet {
	if len(userID) == 0 {
		qs.db.AddError(errors.New("must at least pass one userID in UserIDIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("user_id IN (?)", userID))
}

// UserIDLt is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UserIDLt(userID uint) RepoQuerySet {
	return qs.w(qs.db.Where("user_id < ?", userID))
}

// UserIDLte is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UserIDLte(userID uint) RepoQuerySet {
	return qs.w(qs.db.Where("user_id <= ?", userID))
}

// UserIDNe is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UserIDNe(userID uint) RepoQuerySet {
	return qs.w(qs.db.Where("user_id != ?", userID))
}

// UserIDNotIn is an autogenerated method
// nolint: dupl
func (qs RepoQuerySet) UserIDNotIn(userID ...uint) RepoQuerySet {
	if len(userID) == 0 {
		qs.db.AddError(errors.New("must at least pass one userID in UserIDNotIn"))
		return qs.w(qs.db)
	}
	return qs.w(qs.db.Where("user_id NOT IN (?)", userID))
}

// ===== END of query set RepoQuerySet

// ===== BEGIN of Repo modifiers

// RepoDBSchemaField describes database schema field. It requires for method 'Update'
type RepoDBSchemaField string

// String method returns string representation of field.
// nolint: dupl
func (f RepoDBSchemaField) String() string {
	return string(f)
}

// RepoDBSchema stores db field names of Repo
var RepoDBSchema = struct {
	ID               RepoDBSchemaField
	CreatedAt        RepoDBSchemaField
	UpdatedAt        RepoDBSchemaField
	DeletedAt        RepoDBSchemaField
	UserID           RepoDBSchemaField
	FullName         RepoDBSchemaField
	DisplayFullName  RepoDBSchemaField
	HookID           RepoDBSchemaField
	Provider         RepoDBSchemaField
	ProviderHookID   RepoDBSchemaField
	ProviderID       RepoDBSchemaField
	CommitState      RepoDBSchemaField
	StargazersCount  RepoDBSchemaField
	IsPrivate        RepoDBSchemaField
	CreateFailReason RepoDBSchemaField
}{

	ID:               RepoDBSchemaField("id"),
	CreatedAt:        RepoDBSchemaField("created_at"),
	UpdatedAt:        RepoDBSchemaField("updated_at"),
	DeletedAt:        RepoDBSchemaField("deleted_at"),
	UserID:           RepoDBSchemaField("user_id"),
	FullName:         RepoDBSchemaField("name"),
	DisplayFullName:  RepoDBSchemaField("display_name"),
	HookID:           RepoDBSchemaField("hook_id"),
	Provider:         RepoDBSchemaField("provider"),
	ProviderHookID:   RepoDBSchemaField("provider_hook_id"),
	ProviderID:       RepoDBSchemaField("provider_id"),
	CommitState:      RepoDBSchemaField("commit_state"),
	StargazersCount:  RepoDBSchemaField("stargazers_count"),
	IsPrivate:        RepoDBSchemaField("is_private"),
	CreateFailReason: RepoDBSchemaField("create_fail_reason"),
}

// Update updates Repo fields by primary key
// nolint: dupl
func (o *Repo) Update(db *gorm.DB, fields ...RepoDBSchemaField) error {
	dbNameToFieldName := map[string]interface{}{
		"id":                 o.ID,
		"created_at":         o.CreatedAt,
		"updated_at":         o.UpdatedAt,
		"deleted_at":         o.DeletedAt,
		"user_id":            o.UserID,
		"name":               o.FullName,
		"display_name":       o.DisplayFullName,
		"hook_id":            o.HookID,
		"provider":           o.Provider,
		"provider_hook_id":   o.ProviderHookID,
		"provider_id":        o.ProviderID,
		"commit_state":       o.CommitState,
		"stargazers_count":   o.StargazersCount,
		"is_private":         o.IsPrivate,
		"create_fail_reason": o.CreateFailReason,
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

		return fmt.Errorf("can't update Repo %v fields %v: %s",
			o, fields, err)
	}

	return nil
}

// RepoUpdater is an Repo updates manager
type RepoUpdater struct {
	fields map[string]interface{}
	db     *gorm.DB
}

// NewRepoUpdater creates new Repo updater
// nolint: dupl
func NewRepoUpdater(db *gorm.DB) RepoUpdater {
	return RepoUpdater{
		fields: map[string]interface{}{},
		db:     db.Model(&Repo{}),
	}
}

// ===== END of Repo modifiers

// ===== END of all query sets

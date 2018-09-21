package user

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/golangci/golangci-api/pkg/models"
	"github.com/golangci/golangci-api/pkg/todo/auth/sess"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/events"
	"github.com/golangci/golib/server/context"
	"github.com/jinzhu/gorm"
	"github.com/markbates/goth"
	"github.com/pkg/errors"
)

const userIDSessKey = "UserID"

func updateUserDataIfNeeded(ctx *context.C, u *models.User, gu *goth.User) error {
	var fieldsToUpdate []models.UserDBSchemaField
	if gu.Email != "" && u.Email != gu.Email {
		ctx.L.Infof("Updating user %d email from %s to %s on auth", u.ID, u.Email, gu.Email)
		u.Email = gu.Email
		fieldsToUpdate = append(fieldsToUpdate, models.UserDBSchema.Email)
	}
	if gu.Name != "" && u.Name != gu.Name {
		ctx.L.Infof("Updating user %d name from %s to %s on auth", u.ID, u.Name, gu.Name)
		u.Name = gu.Name
		fieldsToUpdate = append(fieldsToUpdate, models.UserDBSchema.Name)
	}
	if gu.AvatarURL != "" && u.AvatarURL != gu.AvatarURL {
		ctx.L.Infof("Updating user %d avatar from %s to %s on auth", u.ID, u.AvatarURL, gu.AvatarURL)
		u.AvatarURL = gu.AvatarURL
		fieldsToUpdate = append(fieldsToUpdate, models.UserDBSchema.AvatarURL)
	}
	if len(fieldsToUpdate) != 0 {
		if err := u.Update(db.Get(ctx), fieldsToUpdate...); err != nil {
			return fmt.Errorf("can't update user %d fields %v: %s", u.ID, fieldsToUpdate, err)
		}
	}

	return nil
}

func getOrStoreUserInDB(ctx *context.C, gu *goth.User) (*models.User, uint, error) {
	DB := db.Get(ctx)
	var ga models.Auth
	providerUserID, err := strconv.ParseUint(gu.UserID, 10, 64)
	if err != nil {
		return nil, 0, fmt.Errorf("can't parse github user id %q: %s", gu.UserID, err)
	}

	err = models.NewAuthQuerySet(DB).ProviderUserIDEq(providerUserID).OrderDescByID().One(&ga)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, 0, fmt.Errorf("can't select auth with provider user id %d: %s", providerUserID, err)
	}

	if err == gorm.ErrRecordNotFound { // new user, need create it
		name := gu.Name
		if name == "" {
			name = gu.NickName
		}

		u := &models.User{
			Email:     gu.Email,
			Name:      name,
			AvatarURL: gu.AvatarURL,
		}
		if err = u.Create(DB); err != nil {
			return nil, 0, fmt.Errorf("can't create user %v: %s", u, err)
		}

		t := events.NewAuthenticatedTracker(int(u.ID)).WithUserProps(map[string]interface{}{
			"registeredAt": time.Now(),
		})

		go t.Track(ctx.Ctx, "registered", map[string]interface{}{
			"provider": "github",
		})

		return u, 0, nil
	}

	var u models.User
	err = models.NewUserQuerySet(DB).IDEq(ga.UserID).One(&u)
	if err != nil {
		return nil, 0, fmt.Errorf("can't get user with id %d", ga.UserID)
	}

	if err = updateUserDataIfNeeded(ctx, &u, gu); err != nil {
		return nil, 0, fmt.Errorf("can't update user data: %s", err)
	}

	// User already exists
	return &u, ga.ID, nil
}

func LoginGithub(ctx *context.C, gu *goth.User) (err error) {
	finishTx, err := db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("can't start tx: %s", err)
	}
	defer finishTx(&err)

	u, gaID, err := getOrStoreUserInDB(ctx, gu)
	if err != nil {
		return err
	}

	providerUserID, err := strconv.ParseUint(gu.UserID, 10, 64)
	if err != nil {
		return errors.Wrapf(err, "can't parse provider user id %s", gu.UserID)
	}

	rawData, err := json.Marshal(gu.RawData)
	if err != nil {
		return errors.Wrap(err, "json marshal of raw data failed")
	}

	ga := models.Auth{
		Model: gorm.Model{
			ID: gaID,
		},
		RawData:        rawData,
		AccessToken:    gu.AccessToken,
		UserID:         u.ID,
		Login:          gu.NickName,
		Provider:       "github.com",
		ProviderUserID: providerUserID,
	}

	DB := db.Get(ctx)

	if gaID == 0 {
		if err = ga.Create(DB); err != nil {
			return fmt.Errorf("can't create authorization %v: %s", u, err)
		}
	} else {
		err = ga.Update(DB, models.AuthDBSchema.RawData,
			models.AuthDBSchema.AccessToken, models.AuthDBSchema.Login,
		)
		if err != nil {
			return fmt.Errorf("can't update authorization %v: %s", u, err)
		}
	}

	if err := sess.WriteOneValue(ctx.R, ctx.W, userIDSessKey, u.ID); err != nil {
		return fmt.Errorf("can't save user id to session: %s", err)
	}

	return nil
}

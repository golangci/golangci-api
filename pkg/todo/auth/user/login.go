package user

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/app/models"
	"github.com/golangci/golangci-api/pkg/todo/auth/sess"
	"github.com/golangci/golangci-api/pkg/todo/db"
	"github.com/golangci/golangci-api/pkg/todo/events"
	"github.com/golangci/golib/server/context"
	"github.com/jinzhu/gorm"
	"github.com/markbates/goth"
)

const userIDSessKey = "UserID"

func getOrStoreUserInDB(ctx *context.C, gu *goth.User) (*models.User, uint, error) {
	DB := db.Get(ctx)
	var ga models.GithubAuth
	err := models.NewGithubAuthQuerySet(DB).LoginEq(gu.NickName).One(&ga)
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, 0, fmt.Errorf("can't select user with nickname %q: %s", gu.NickName, err)
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

	ga := models.GithubAuth{
		Model: gorm.Model{
			ID: gaID,
		},
		RawData:     gu.RawData,
		AccessToken: gu.AccessToken,
		UserID:      u.ID,
		Login:       gu.NickName,
	}

	DB := db.Get(ctx)

	if gaID == 0 {
		if err = ga.Create(DB); err != nil {
			return fmt.Errorf("can't create github authorization %v: %s", u, err)
		}
	} else {
		err = ga.Update(DB, "raw_data", models.GithubAuthDBSchema.AccessToken)
		if err != nil {
			return fmt.Errorf("can't create github authorization %v: %s", u, err)
		}
	}

	if err := sess.WriteOneValue(ctx, userIDSessKey, u.ID); err != nil {
		return fmt.Errorf("can't save user id to session: %s", err)
	}

	return nil
}

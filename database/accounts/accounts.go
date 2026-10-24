package accounts

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const constantSalt = "06Wm4Jv1Hkxx"

// CheckPassword 检查密码，正确时返回用户 UUID 和 true
func CheckPassword(username, passwd string) (uuid string, ok bool) {
	db := dbcore.GetDBInstance()
	var user models.User
	result := db.Where("username = ?", username).First(&user)
	if result.Error != nil {
		return "", false
	}
	if strings.HasPrefix(user.Passwd, passwordHashPrefix) {
		if bcrypt.CompareHashAndPassword([]byte(strings.TrimPrefix(user.Passwd, passwordHashPrefix)), passwordInput(passwd)) != nil {
			return "", false
		}
	} else {
		// Upgrade old accounts only after a successful constant-time verification.
		legacy := sha256.Sum256([]byte(passwd + constantSalt))
		if subtle.ConstantTimeCompare([]byte(base64.StdEncoding.EncodeToString(legacy[:])), []byte(user.Passwd)) != 1 {
			return "", false
		}
		encoded, err := hashPasswd(passwd)
		if err != nil {
			return "", false
		}
		updated := db.Model(&models.User{}).Where("uuid = ? AND passwd = ?", user.UUID, user.Passwd).Update("passwd", encoded)
		if updated.Error != nil || updated.RowsAffected != 1 {
			return "", false
		}
	}
	return user.UUID, true
}

// ForceResetPassword 强制重置用户密码
func ForceResetPassword(username, passwd string) error {
	db := dbcore.GetDBInstance()
	encoded, err := hashPasswd(passwd)
	if err != nil {
		return err
	}
	result := db.Model(&models.User{}).Where("username = ?", username).Update("passwd", encoded)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("无法找到用户名")
	}
	return nil
}

const passwordHashPrefix = "$may-bcrypt-sha256$"

// Prehashing avoids bcrypt's 72-byte truncation while retaining a random salt
// and an adaptive work factor. The fixed-salt SHA256 format is read-only legacy.
func passwordInput(passwd string) []byte {
	h := sha256.Sum256([]byte(passwd))
	return []byte(base64.StdEncoding.EncodeToString(h[:]))
}

func hashPasswd(passwd string) (string, error) {
	h, err := bcrypt.GenerateFromPassword(passwordInput(passwd), 12)
	if err != nil {
		return "", err
	}
	return passwordHashPrefix + string(h), nil
}

func CreateAccount(username, passwd string) (user models.User, err error) {
	return CreateAccountWithDB(dbcore.GetDBInstance(), username, passwd)
}

func CreateAccountWithDB(db *gorm.DB, username, passwd string) (models.User, error) {
	encoded, err := hashPasswd(passwd)
	if err != nil {
		return models.User{}, err
	}
	user := models.User{
		UUID:     uuid.New().String(),
		Username: username,
		Passwd:   encoded,
	}
	return user, db.Create(&user).Error
}

func DeleteAccountByUsername(username string) (err error) {
	return DeleteAccountByUsernameWithDB(dbcore.GetDBInstance(), username)
}

func DeleteAccountByUsernameWithDB(db *gorm.DB, username string) error {
	return db.Where("username = ?", username).Delete(&models.User{}).Error
}

func GetUserByUUID(uuid string) (models.User, error) {
	var user models.User
	return user, dbcore.GetDBInstance().Where("uuid = ?", uuid).First(&user).Error
}

// GetUserBySSO 通过 SSO 信息获取用户
func GetUserBySSO(ssoID string) (models.User, error) {
	db := dbcore.GetDBInstance()
	var user models.User
	if err := db.Where("sso_id = ?", ssoID).First(&user).Error; err != nil {
		return models.User{}, fmt.Errorf("用户不存在：%s", ssoID)
	}
	return user, nil
}

func BindingExternalAccount(uuid string, sso_id string) error {
	db := dbcore.GetDBInstance()
	return db.Model(&models.User{}).Where("uuid = ?", uuid).Update("sso_id", sso_id).Error
}

func UnbindExternalAccount(uuid string) error {
	db := dbcore.GetDBInstance()
	return db.Model(&models.User{}).Where("uuid = ?", uuid).Update("sso_id", "").Error
}

func UpdateUser(uuid string, name, password, sso_type *string) error {
	db := dbcore.GetDBInstance()
	// Check if user exists
	var existingUser models.User
	result := db.Where("uuid = ?", uuid).First(&existingUser)
	if result.Error != nil {
		return fmt.Errorf("user not found: %s", uuid)
	}
	updates := make(map[string]interface{})
	if name != nil {
		updates["username"] = *name
	}
	if password != nil {
		encoded, err := hashPasswd(*password)
		if err != nil {
			return err
		}
		updates["passwd"] = encoded
	}
	if sso_type != nil {
		updates["sso_type"] = *sso_type
	}
	updates["updated_at"] = time.Now().UTC()
	if err := db.Model(&models.User{}).Where("uuid = ?", uuid).Updates(updates).Error; err != nil {
		return err
	}
	if password != nil {
		DeleteAllSessions()
	}
	return nil
}

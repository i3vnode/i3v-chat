// Package basic is an authenticator by login-password.
package basic

import (
	"encoding/json"
	"errors"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/i3vnode/i3v-chat/server/auth"
	"github.com/i3vnode/i3v-chat/server/logs"
	"github.com/i3vnode/i3v-chat/server/store"
	"github.com/i3vnode/i3v-chat/server/store/types"

	"golang.org/x/crypto/bcrypt"
)

// Define default constraints on login and password
const (
	defaultMinLoginLength = 2
	defaultMaxLoginLength = 32

	defaultMinPasswordLength = 3
)

const (
	validatorName = "email"

	// maxRetries  = 4
	// defaultPort = "25"

	// Technically email could be up to 255 bytes long but practically 128 is enough.
	// maxEmailLength = 128

	// codeLength = log10(maxCodeValue)
	codeLength   = 6
	maxCodeValue = 1000000

	// Email template parts
	// emailSubject   = "subject"
	// emailBodyPlain = "body_plain"
	// emailBodyHTML  = "body_html"
)

// Token suitable as a login: starts and ends with a Unicode letter (class L) or number (class N),
// contains Unicode letters, numbers, dot and underscore.
var loginPattern = regexp.MustCompile(`^[\pL\pN][_.\pL\pN]*[\pL\pN]+$`)

// authenticator is the type to map authentication methods to.
type authenticator struct {
	name      string
	addToTags bool

	minPasswordLength int
	minLoginLength    int
}

type Envelope struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Result  struct {
		ActivitiSync   interface{} `json:"activitiSync"`
		Authentication interface{} `json:"authentication"`
		Avatar         string      `json:"avatar"`
		Birthday       interface{} `json:"birthday"`
		ClientID       interface{} `json:"clientId"`
		CreateBy       interface{} `json:"createBy"`
		CreateTime     string      `json:"createTime"`
		DelFlag        int64       `json:"delFlag"`
		DepartIds      interface{} `json:"departIds"`
		DepartureTime  interface{} `json:"departureTime"`
		Email          interface{} `json:"email"`
		ID             string      `json:"id"`
		InductionTime  interface{} `json:"inductionTime"`
		Integral       interface{} `json:"integral"`
		LoginTime      string      `json:"loginTime"`
		Nickname       interface{} `json:"nickname"`
		OrgCode        string      `json:"orgCode"`
		OrgCodeTxt     interface{} `json:"orgCodeTxt"`
		Password       string      `json:"password"`
		Permissions    interface{} `json:"permissions"`
		Phone          string      `json:"phone"`
		Post           interface{} `json:"post"`
		Posts          interface{} `json:"posts"`
		QqNo           interface{} `json:"qqNo"`
		QqOpenid       interface{} `json:"qqOpenid"`
		Realname       string      `json:"realname"`
		RegularTime    interface{} `json:"regularTime"`
		RelTenantIds   interface{} `json:"relTenantIds"`
		Remark         interface{} `json:"remark"`
		Roles          interface{} `json:"roles"`
		Sex            int64       `json:"sex"`
		State          interface{} `json:"state"`
		Station        interface{} `json:"station"`
		Status         int64       `json:"status"`
		Telephone      interface{} `json:"telephone"`
		TenantIDNow    string      `json:"tenantIdNow"`
		TenantName     interface{} `json:"tenantName"`
		Type           int64       `json:"type"`
		UpdateBy       string      `json:"updateBy"`
		UpdateTime     string      `json:"updateTime"`
		UserIdentity   interface{} `json:"userIdentity"`
		UserTenants    interface{} `json:"userTenants"`
		UserType       interface{} `json:"userType"`
		Username       string      `json:"username"`
		Userno         string      `json:"userno"`
		WorkNo         interface{} `json:"workNo"`
		WxMpOpenid     interface{} `json:"wxMpOpenid"`
		WxUnionid      interface{} `json:"wxUnionid"`
	} `json:"result"`
	Success   bool  `json:"success"`
	Timestamp int64 `json:"timestamp"`
}

type UserEnv struct {
	Basic  string      `json:"basic"`
	Email  string      `json:"email"`
	Public interface{} `json:"public"` // 结构体类数组
}

func (a *authenticator) checkLoginPolicy(uname string) error {
	rlogin := []rune(uname)
	if len(rlogin) < a.minLoginLength || len(rlogin) > defaultMaxLoginLength || !loginPattern.MatchString(uname) {
		return types.ErrPolicy
	}

	return nil
}

func (a *authenticator) checkPasswordPolicy(password string) error {
	if len([]rune(password)) < a.minPasswordLength {
		return types.ErrPolicy
	}

	return nil
}

func parseSecret(bsecret []byte) (uname, password string, err error) {
	secret := string(bsecret)

	splitAt := strings.Index(secret, ":")
	if splitAt < 0 {
		err = types.ErrMalformed
		return
	}

	uname = strings.ToLower(secret[:splitAt])
	password = secret[splitAt+1:]
	return
}

// Init initializes the basic authenticator.
func (a *authenticator) Init(jsonconf json.RawMessage, name string) error {
	if name == "" {
		return errors.New("auth_basic: authenticator name cannot be blank")
	}

	if a.name != "" {
		return errors.New("auth_basic: already initialized as " + a.name + "; " + name)
	}

	type configType struct {
		// AddToTags indicates that the user name should be used as a searchable tag.
		AddToTags         bool `json:"add_to_tags"`
		MinPasswordLength int  `json:"min_password_length"`
		MinLoginLength    int  `json:"min_login_length"`
	}

	var config configType
	if err := json.Unmarshal(jsonconf, &config); err != nil {
		return errors.New("auth_basic: failed to parse config: " + err.Error() + "(" + string(jsonconf) + ")")
	}
	a.name = name
	a.addToTags = config.AddToTags
	a.minPasswordLength = config.MinPasswordLength
	if a.minPasswordLength <= 0 {
		a.minPasswordLength = defaultMinPasswordLength
	}
	a.minLoginLength = config.MinLoginLength
	if a.minLoginLength > defaultMaxLoginLength {
		return errors.New("auth_basic: min_login_length exceeds the limit")
	}
	if a.minLoginLength <= 0 {
		a.minLoginLength = defaultMinLoginLength
	}

	return nil
}

// IsInitialized returns true if the handler is initialized.
func (a *authenticator) IsInitialized() bool {
	return a.name != ""
}

// AddRecord adds a basic authentication record to DB.
func (a *authenticator) AddRecord(rec *auth.Rec, secret []byte, remoteAddr string) (*auth.Rec, error) {
	uname, password, err := parseSecret(secret)
	if err != nil {
		return nil, err
	}

	if err = a.checkLoginPolicy(uname); err != nil {
		return nil, err
	}

	if err = a.checkPasswordPolicy(password); err != nil {
		return nil, err
	}

	passhash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	var expires time.Time
	if rec.Lifetime > 0 {
		expires = time.Now().Add(time.Duration(rec.Lifetime)).UTC().Round(time.Millisecond)
	}

	authLevel := rec.AuthLevel
	if authLevel == auth.LevelNone {
		authLevel = auth.LevelAuth
	}

	err = store.Users.AddAuthRecord(rec.Uid, rec.UUid, authLevel, a.name, uname, passhash, expires)
	if err != nil {
		return nil, err
	}

	rec.AuthLevel = authLevel
	if a.addToTags {
		rec.Tags = append(rec.Tags, a.name+":"+uname)
	}
	return rec, nil
}

// UpdateRecord updates password for basic authentication.
func (a *authenticator) UpdateRecord(rec *auth.Rec, secret []byte, remoteAddr string) (*auth.Rec, error) {
	uname, password, err := parseSecret(secret)
	if err != nil {
		return nil, err
	}

	login, _, authLevel, _, _, err := store.Users.GetAuthRecord(rec.Uid, a.name)
	if err != nil {
		return nil, err
	}
	// User does not have a record.
	if login == "" {
		return nil, types.ErrNotFound
	}

	if uname == "" || uname == login {
		// User is changing just the password.
		uname = login
	} else if err = a.checkLoginPolicy(uname); err != nil {
		return nil, err
	} else if uid, _, _, _, err := store.Users.GetAuthUniqueRecord(a.name, uname); err != nil {
		return nil, err
	} else if !uid.IsZero() {
		// The (new) user name already exists. Report an error.
		return nil, types.ErrDuplicate
	}

	if err = a.checkPasswordPolicy(password); err != nil {
		return nil, err
	}

	passhash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, types.ErrInternal
	}
	var expires time.Time
	if rec.Lifetime > 0 {
		expires = types.TimeNow().Add(time.Duration(rec.Lifetime))
	}
	err = store.Users.UpdateAuthRecord(rec.Uid, authLevel, a.name, uname, passhash, expires)
	if err != nil {
		return nil, err
	}

	// Remove old tag from the list of tags
	oldTag := a.name + ":" + login
	for i, tag := range rec.Tags {
		if tag == oldTag {
			rec.Tags[i] = rec.Tags[len(rec.Tags)-1]
			rec.Tags = rec.Tags[:len(rec.Tags)-1]

			break
		}
	}
	// Add new tag
	rec.Tags = append(rec.Tags, a.name+":"+uname)

	return rec, nil
}

// Authenticate checks login and password.
func (a *authenticator) Authenticate(secret []byte, remoteAddr string) (*auth.Rec, []byte, error) {
	uname, password, err := parseSecret(secret)
	if err != nil {
		return nil, nil, err
	}

	uid, authLvl, passhash, expires, err := store.Users.GetAuthUniqueRecord(a.name, uname)
	if err != nil {
		return nil, nil, err
	}
	if uid.IsZero() {
		// Invalid login.
		return nil, nil, types.ErrFailed
	}
	if !expires.IsZero() && expires.Before(time.Now()) {
		// The record has expired
		return nil, nil, types.ErrExpired
	}

	err = bcrypt.CompareHashAndPassword(passhash, []byte(password))
	if err != nil {
		// Invalid password
		return nil, nil, types.ErrFailed
	}

	var lifetime time.Duration
	if !expires.IsZero() {
		lifetime = time.Until(expires)
	}
	return &auth.Rec{
		Uid:       uid,
		AuthLevel: authLvl,
		Lifetime:  auth.Duration(lifetime),
		Features:  0,
		State:     types.StateUndefined}, nil, nil
}

// Authenticate checks Token.
func (a *authenticator) AuthenticateToken(secret []byte, remoteAddr string) (*auth.Rec, []byte, error) {

	scheme := "basic"
	uname, utoken, err := parseSecret(secret)
	if err != nil {
		return nil, nil, err
	}
	//uname = "13258954786"

	uuname := scheme + ":" + uname

	//utoken = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJsb2dpblR5cGUiOjEsImV4cCI6MTY3MjM0Mzc4NiwidXNlcm5hbWUiOiIxMzI1ODk1NDc4NiJ9.tl5pz509mdG-CW7Bf89F1v8qk7TqcC68Hbjne63KM74"

	uid, uuid, authLvl, _, expires, err := store.Users.GetAuthUniqueRecordByToken(utoken, uuname)
	if err != nil {
		logs.Warn.Println("GetAuthUniqueRecordByToken err:" + err.Error())
		return nil, nil, err
	}

	var lifetime time.Duration
	if !expires.IsZero() {
		lifetime = time.Until(expires)
	}

	//logs.Warn.Println("GetAuthUniqueRecordByToken uuid:" + uuid)

	if uuid != "" {
		return &auth.Rec{
			Uid:       uid,
			UUid:      uuid,
			AuthLevel: authLvl,
			Lifetime:  auth.Duration(lifetime),
			Features:  0,
			State:     types.StateUndefined}, nil, nil
	}

	var url = "http://saas.i3vsoft.com:9112/saas-1.1/sys/verifyLoginToken"

	url = url + "?user_name=" + uname + "&token=" + utoken
	//req_data := `{ "token": "` + utoken + `", "user_name": "` + uname + `"}`

	//req_body := strings.NewReader(req_data)

	//resp, err := sendRequest(url, req_body, nil, "GET")
	respon, err := sendRequest(url, nil, nil, "GET")

	if err != nil {
		return nil, nil, err
	}

	logs.Warn.Println("login SASS log. " + string(respon))

	//Judge whether the access is successful
	var env Envelope

	if err := json.Unmarshal(respon, &env); err != nil {
		logs.Warn.Println(err.Error())
		return nil, nil, err
	}

	if !env.Success {
		err := errors.New("saas login error")
		logs.Warn.Println("saas login error! token: ", utoken)
		return nil, nil, err
	}

	logs.Warn.Println("login AddAuthTokenRecord log. uuid:" + env.Result.ID + ", a.name:" + a.name + ",uname:" + uname + ",utoken:" + utoken)

	// Find authenticator for the requested scheme.
	authhdl := store.Store.GetLogicalAuthHandler(scheme)
	if authhdl == nil {
		// New accounts must have an authentication scheme
		// s.queueOut(ErrMalformed(msg.Id, "", msg.Timestamp))
		logs.Warn.Println("create user: unknown auth handler")
		return nil, nil, err
	}

	var user types.User
	var private interface{}

	// Assign default access values in case the acc creator has not provided them
	user.Access.Auth = getDefaultAccess(types.TopicCatP2P, true, false) |
		getDefaultAccess(types.TopicCatGrp, true, false)
	user.Access.Anon = getDefaultAccess(types.TopicCatP2P, false, false) |
		getDefaultAccess(types.TopicCatGrp, false, false)

	strEmail := string(interfaceToBytes(env.Result.Email))
	if strEmail == "" {
		strEmail = "null@email.com"
	}

	jbyte := `{"basic":"basic:` + env.Result.Username + `","email":"email:` + strEmail + `", "public":{"fn":"` + env.Result.Username + `"}}`
	var stu UserEnv
	if err = json.Unmarshal([]byte(jbyte), &stu); err != nil {
		logs.Warn.Println("create user: failed to create user public ", err)
		return nil, nil, err
	}

	user.Public = stu.Public

	var dst []string
	dst = append(dst, stu.Basic)
	dst = append(dst, stu.Email)
	user.Tags = types.StringSlice(dst)

	// Create user record in the database.
	if _, err := store.Users.Create(&user, private); err != nil {
		logs.Warn.Println("create user: failed to create user", err)
		return nil, nil, err
	}

	// Add authentication record. The authhdl.AddRecord may change tags.
	logs.Warn.Println("create user: add auth record user.id:"+user.Id+",UUid:", user.Useruuid)
	passhash := []byte("")
	authLevel := auth.LevelAuth
	err = store.Users.AddAuthTokenRecord(user.Uid(), user.Useruuid, authLevel, a.name, uname, passhash, utoken, expires)
	if err != nil {
		return nil, nil, err
	}

	var resp string
	// Generate expected response as a random numeric string between 0 and 999999.
	// The PRNG is already initialized in main.go. No need to initialize it here again.
	resp = strconv.FormatInt(int64(rand.Intn(maxCodeValue)), 10)
	resp = strings.Repeat("0", codeLength-len(resp)) + resp

	// Create or update validation record in DB.
	_, err = store.Users.UpsertCred(&types.Credential{
		User:   user.Uid().String(),
		Method: validatorName,
		Value:  strEmail,
		Resp:   resp})
	if err != nil {
		return nil, nil, err
	}

	// Auto Confirm Cred
	store.Users.ConfirmCred(user.Uid(), validatorName)

	return &auth.Rec{
		Uid:       user.Uid(),
		AuthLevel: authLevel,
		Lifetime:  auth.Duration(lifetime),
		Features:  0,
		State:     types.StateUndefined}, nil, nil

}

func interfaceToBytes(in interface{}) []byte {
	if in != nil {
		out, _ := json.Marshal(in)
		return out
	}
	return nil
}

// Get default modeWant for the given topic category
func getDefaultAccess(cat types.TopicCat, authUser, isChan bool) types.AccessMode {
	if !authUser {
		return types.ModeNone
	}

	switch cat {
	case types.TopicCatP2P:
		return types.ModeCP2P
	case types.TopicCatFnd:
		return types.ModeNone
	case types.TopicCatGrp:
		if isChan {
			return types.ModeCChnWriter
		}
		return types.ModeCPublic
	case types.TopicCatMe:
		return types.ModeCSelf
	default:
		panic("Unknown topic category")
	}
}

// AsTag convert search token into a prefixed tag, if possible.
func (a *authenticator) AsTag(token string) string {
	if !a.addToTags {
		return ""
	}

	if err := a.checkLoginPolicy(token); err != nil {
		return ""
	}

	return a.name + ":" + token
}

// IsUnique checks login uniqueness.
func (a *authenticator) IsUnique(secret []byte, remoteAddr string) (bool, error) {
	uname, _, err := parseSecret(secret)
	if err != nil {
		return false, err
	}

	if err := a.checkLoginPolicy(uname); err != nil {
		return false, err
	}

	uid, _, _, _, err := store.Users.GetAuthUniqueRecord(a.name, uname)
	if err != nil {
		return false, err
	}

	if uid.IsZero() {
		return true, nil
	}
	return false, types.ErrDuplicate
}

// GenSecret is not supported, generates an error.
func (authenticator) GenSecret(rec *auth.Rec) ([]byte, time.Time, error) {
	return nil, time.Time{}, types.ErrUnsupported
}

// DelRecords deletes saved authentication records of the given user.
func (a *authenticator) DelRecords(uid types.Uid) error {
	return store.Users.DelAuthRecords(uid, a.name)
}

// RestrictedTags returns tag namespaces (prefixes) restricted by this adapter.
func (a *authenticator) RestrictedTags() ([]string, error) {
	var prefix []string
	if a.addToTags {
		prefix = []string{a.name}
	}
	return prefix, nil
}

// GetResetParams returns authenticator parameters passed to password reset handler.
func (a *authenticator) GetResetParams(uid types.Uid) (map[string]interface{}, error) {
	login, _, _, _, _, err := store.Users.GetAuthRecord(uid, a.name)
	if err != nil {
		return nil, err
	}
	// User does not have a record matching the authentication scheme.
	if login == "" {
		return nil, types.ErrNotFound
	}

	params := make(map[string]interface{})
	params["login"] = login
	return params, nil
}

const realName = "basic"

// GetRealName returns the hardcoded name of the authenticator.
func (authenticator) GetRealName() string {
	return realName
}

func init() {
	store.RegisterAuthScheme(realName, &authenticator{})
}

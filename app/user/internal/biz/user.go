package biz

import (
	"context"
	"time"

	v1 "shared-device-saas/api/user/v1"
	"shared-device-saas/pkg/auth"
	"shared-device-saas/pkg/redis"
	"shared-device-saas/pkg/sms"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserNotFound      = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "用户不存在")
	ErrUserAlreadyExists = errors.Conflict(v1.ErrorReason_USER_ALREADY_EXISTS.String(), "用户已存在")
	ErrInvalidPassword   = errors.Unauthorized(v1.ErrorReason_INVALID_PASSWORD.String(), "密码错误")
	ErrInvalidToken      = errors.Unauthorized(v1.ErrorReason_INVALID_TOKEN.String(), "令牌无效")
	ErrUserDisabled      = errors.Forbidden(v1.ErrorReason_USER_DISABLED.String(), "用户已禁用")
	ErrSMSCodeInvalid    = errors.BadRequest(v1.ErrorReason_SMS_CODE_INVALID.String(), "验证码不正确")
	ErrSMSCodeExpired    = errors.BadRequest(v1.ErrorReason_SMS_CODE_EXPIRED.String(), "验证码已过期")
	ErrSMSSendLimit      = errors.BadRequest(v1.ErrorReason_SMS_SEND_LIMIT.String(), "短信发送频率超限")
	ErrPasswordLocked    = errors.Forbidden(v1.ErrorReason_PASSWORD_LOCKED.String(), "密码错误次数过多，已被锁定")
	ErrInvalidPhone      = errors.BadRequest("INVALID_PHONE", "手机号格式不正确")
	ErrPasswordNotMatch  = errors.BadRequest(v1.ErrorReason_PASSWORD_NOT_MATCH.String(), "两次密码不一致")
	ErrInvalidEmail      = errors.BadRequest("INVALID_EMAIL", "邮箱格式不正确")
	ErrEmailAlreadyUsed  = errors.Conflict("EMAIL_ALREADY_USED", "邮箱已被使用")
	ErrNoLoginAccount    = errors.BadRequest("NO_LOGIN_ACCOUNT", "请输入手机号或邮箱")
)

// User 用户实体
type User struct {
	ID         string // ObjectID Hex（账号）
	Password   string // bcrypt 加密后的密码
	Email      string // 邮箱（可选）
	Phone      string // 手机号
	Nickname   string // 昵称（默认：用户_手机号后4位）
	Avatar     string // 头像URL
	InviteCode string // 邀请码
	Status     int32  // 状态（1正常，0禁用）
	Role       string // 角色（user/admin）
	CreateTime string // 创建时间（RFC3339格式）
	UpdateTime string // 更新时间（RFC3339格式）
}

// TokenPair Token对
type TokenPair struct {
	AccessToken  string // 访问令牌
	RefreshToken string // 刷新令牌
	ExpiresIn    int64  // 过期时间（秒）
}

// UserRepo 用户仓储接口
type UserRepo interface {
	Create(context.Context, *User) (*User, error)       // 创建用户
	FindByPhone(context.Context, string) (*User, error) // 手机号查找
	FindByEmail(context.Context, string) (*User, error) // 新增：邮箱查找
	FindByID(context.Context, string) (*User, error)    // 根据ID查找用户
	Update(context.Context, *User) (*User, error)       // 更新用户
}

// UserUsecase 用户用例
type UserUsecase struct {
	repo       UserRepo         // 用户仓储接口
	redis      *redis.Client    // Redis 客户端
	sms        *sms.IhuyiClient // 短信服务客户端
	jwtManager *auth.JWTManager // JWT 管理器
	log        *log.Helper      // 日志助手
}

// NewUserUsecase 创建用户用例
func NewUserUsecase(repo UserRepo, redis *redis.Client, sms *sms.IhuyiClient, jwtManager *auth.JWTManager, logger log.Logger) *UserUsecase {
	return &UserUsecase{
		repo:       repo,                  // 用户仓储接口
		redis:      redis,                 // Redis 客户端
		sms:        sms,                   // 短信服务客户端
		jwtManager: jwtManager,            // JWT 管理器
		log:        log.NewHelper(logger), // 日志助手
	}
}

// generateDefaultNickname 生成默认昵称：用户_手机号后4位
func generateDefaultNickname(phone string) string {
	if len(phone) < 4 {
		return "用户_" + phone
	}
	return "用户_" + phone[len(phone)-4:]
}

// ========================================
// 1. SendSms 发送短信验证码
// ========================================

func (uc *UserUsecase) SendSms(ctx context.Context, phone string) (int64, error) {
	// 1. 校验手机号格式
	if !sms.ValidatePhone(phone) {
		return 0, ErrInvalidPhone
	}

	// 2. 校验发送冷却（60秒内不能重复发送）
	cooldown, err := uc.redis.CheckSMSCooldown(ctx, phone)
	if err != nil {
		uc.log.Errorf("Check SMS cooldown: failed %v", err)
	}
	if cooldown {
		return 0, ErrSMSSendLimit
	}

	// 3. 校验日发送次数（一天最多10次）
	count, err := uc.redis.GetSMSCount(ctx, phone)
	if err != nil {
		uc.log.Errorf("Get SMS count failed: %v", err)
	}
	if count >= 10 {
		return 0, ErrSMSSendLimit
	}

	// 4. 发送短信验证码
	code, err := uc.sms.SendCode(phone)
	if err != nil {
		return 0, err
	}

	// 5. 存储验证码到 Redis（5分钟过期）
	if err := uc.redis.SetSMSCode(ctx, phone, code); err != nil {
		uc.log.Errorf("Set SMS code failed: %v", err)
		return 0, err
	}

	// 6. 设置发送冷却
	if err := uc.redis.SetSMSCooldown(ctx, phone); err != nil {
		uc.log.Errorf("Set SMS cooldown failed: %v", err)
	}

	// 7. 增加发送次数
	uc.redis.IncrSMSCount(ctx, phone)

	// 8. 返回过期时间（5分钟 = 300秒）
	expire := int64(300)
	uc.log.Infof("SMS sent: phone=%s, expire=%d", phone, expire)
	return expire, nil
}

// ========================================
// 2. Register 注册（手机号 + 验证码 + 密码 + 确认密码）
// ========================================

func (uc *UserUsecase) Register(ctx context.Context, phone, smsCode, password, confirmPassword, inviteCode string) (string, error) {
	// 1. 校验手机号是否已注册
	existing, err := uc.repo.FindByPhone(ctx, phone)
	if err == nil && existing != nil {
		return "", ErrUserAlreadyExists
	}

	// 2. 校验验证码
	storedCode, err := uc.redis.GetSMSCode(ctx, phone)
	if err != nil {
		uc.log.Errorf("Get SMS code failed: %v", err)
		return "", err
	}
	if storedCode == "" {
		return "", ErrSMSCodeExpired
	}
	if storedCode != smsCode {
		return "", ErrSMSCodeInvalid
	}

	// 3. 删除验证码（验证通过）
	uc.redis.DelSMSCode(ctx, phone)

	// 4. 校验两次密码一致
	if password != confirmPassword {
		return "", ErrPasswordNotMatch
	}

	// 5. bcrypt 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		uc.log.Errorf("Hash password failed: %v", err)
		return "", err
	}

	// 6. 设置默认值
	now := time.Now().Format(time.RFC3339)
	user := &User{
		Phone:      phone,
		Password:   string(hashedPassword),
		Nickname:   generateDefaultNickname(phone), // 自动生成昵称：用户_手机号后4位
		InviteCode: inviteCode,
		Status:     1,
		Role:       "user",
		CreateTime: now,
		UpdateTime: now,
	}

	// 7. 创建用户
	created, err := uc.repo.Create(ctx, user)
	if err != nil {
		return "", err
	}

	uc.log.Infof("User registered: id=%s, phone=%s, nickname=%s", created.ID, created.Phone, created.Nickname)
	return created.ID, nil // 返回账号（user_id）
}

// ========================================
// 3. LoginByPwd 密码登录（手机号或邮箱）
// ========================================

func (uc *UserUsecase) LoginByPwd(ctx context.Context, phone, email, password string) (*User, *TokenPair, error) {
	// 1. 判断登录账号类型
	var user *User
	var err error

	if phone != "" {
		// 手机号登录
		user, err = uc.repo.FindByPhone(ctx, phone)
	} else if email != "" {
		// 邮箱登录
		user, err = uc.repo.FindByEmail(ctx, email)
	} else {
		// 两个都为空
		return nil, nil, ErrNoLoginAccount
	}

	if err != nil || user == nil {
		return nil, nil, ErrUserNotFound
	}

	// 2. 检查用户状态
	if user.Status != 1 {
		return nil, nil, ErrUserDisabled
	}

	// 3. 检查密码是否被锁定
	locked, err := uc.redis.CheckPwdLocked(ctx, user.Phone)
	if err != nil {
		uc.log.Errorf("Check password locked failed: %v", err)
	}
	if locked {
		return nil, nil, ErrPasswordLocked
	}

	// 4. bcrypt 校验密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		// 密码错误，增加错误计数
		count, _ := uc.redis.IncrPwdErrCount(ctx, user.Phone)
		uc.log.Infof("Password error count: phone=%s, count=%d", user.Phone, count)

		// 超过5次，锁定15分钟
		if count >= 5 {
			uc.redis.SetPwdLock(ctx, user.Phone)
			uc.log.Warnf("Password locked: phone=%s", user.Phone)
			return nil, nil, ErrPasswordLocked
		}

		return nil, nil, ErrInvalidPassword
	}

	// 5. 密码正确，清除错误计数
	uc.redis.DelPwdErrCount(ctx, user.Phone)

	// 6. 生成 JWT Token
	sessionID := auth.NewJTI()
	tokenPair, err := uc.jwtManager.GenerateTokenPair(
		user.ID,
		0,
		sessionID,
		"",
		[]string{user.Role},
	)
	if err != nil {
		uc.log.Errorf("Generate token failed: %v", err)
		return nil, nil, err
	}

	// 7. 存储 Token Session 到 Redis（7天）
	refreshTTL := 7 * 24 * time.Hour
	uc.redis.SetSession(ctx, sessionID, user.ID, refreshTTL) // 存储 Token Session 到 Redis（7天）

	uc.log.Infof("User logged in by password: id=%s, phone=%s, nickname=%s", user.ID, user.Phone, user.Nickname)
	return user, &TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// ========================================
// 4. LoginBySms 短信验证码登录（自动注册）
// ========================================

func (uc *UserUsecase) LoginBySms(ctx context.Context, phone, code string) (*User, *TokenPair, error) {
	// 1. 校验验证码
	storedCode, err := uc.redis.GetSMSCode(ctx, phone)
	if err != nil {
		uc.log.Errorf("Get SMS code failed: %v", err)
		return nil, nil, err
	}
	if storedCode == "" {
		return nil, nil, ErrSMSCodeExpired
	}
	if storedCode != code {
		return nil, nil, ErrSMSCodeInvalid
	}

	// 2. 删除验证码（验证通过）
	uc.redis.DelSMSCode(ctx, phone)

	// 3. 查询用户
	user, err := uc.repo.FindByPhone(ctx, phone)

	// 4. 用户不存在，自动注册
	if err != nil || user == nil {
		now := time.Now().Format(time.RFC3339)
		newUser := &User{
			Phone:      phone,
			Nickname:   generateDefaultNickname(phone), // 自动生成昵称
			Status:     1,
			Role:       "user",
			CreateTime: now,
			UpdateTime: now,
		}
		user, err = uc.repo.Create(ctx, newUser)
		if err != nil {
			return nil, nil, err
		}
		uc.log.Infof("Auto registered user: id=%s, phone=%s, nickname=%s", user.ID, user.Phone, user.Nickname)
	}

	// 5. 检查用户状态
	if user.Status != 1 {
		return nil, nil, ErrUserDisabled
	}

	// 6. 生成 JWT Token
	sessionID := auth.NewJTI()
	tokenPair, err := uc.jwtManager.GenerateTokenPair(
		user.ID,
		0,
		sessionID,
		"",
		[]string{user.Role},
	)
	if err != nil {
		uc.log.Errorf("Generate token failed: %v", err)
		return nil, nil, err
	}

	// 7. 存储 Token Session
	refreshTTL := 7 * 24 * time.Hour
	uc.redis.SetSession(ctx, sessionID, user.ID, refreshTTL)

	uc.log.Infof("User logged in by SMS: id=%s, phone=%s, nickname=%s", user.ID, user.Phone, user.Nickname)
	return user, &TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// ========================================
// 5. RefreshToken 刷新 Token
// ========================================

func (uc *UserUsecase) RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error) {
	// 1. 解析 refresh token
	claims, err := uc.jwtManager.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// 2. 检查 Token 黑名单
	blacklisted, err := uc.redis.CheckTokenBlack(ctx, claims.ID)
	if err != nil {
		uc.log.Errorf("Check token black failed: %v", err)
	}
	if blacklisted {
		return nil, ErrInvalidToken
	}

	// 3. 生成新的 Token
	newSessionID := auth.NewJTI()
	tokenPair, err := uc.jwtManager.GenerateTokenPair(
		claims.UserID,
		claims.TenantID,
		newSessionID,
		claims.DeviceID,
		claims.Roles,
	)
	if err != nil {
		uc.log.Errorf("Generate token failed: %v", err)
		return nil, err
	}

	// 4. 将旧 Token 加入黑名单
	refreshTTL := 7 * 24 * time.Hour
	uc.redis.SetTokenBlack(ctx, claims.ID, refreshTTL)

	// 5. 存储新 Session
	uc.redis.SetSession(ctx, newSessionID, claims.UserID, refreshTTL)

	uc.log.Infof("Token refreshed: userID=%s", claims.UserID)
	return &TokenPair{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// ========================================
// 6. Logout 退出登录
// ========================================

func (uc *UserUsecase) Logout(ctx context.Context, refreshToken string) (bool, string, error) {
	// 1. 解析 refresh token
	claims, err := uc.jwtManager.ParseRefreshToken(refreshToken)
	if err != nil {
		return false, "invalid token", nil
	}

	// 2. 将 Token 加入黑名单
	refreshTTL := 7 * 24 * time.Hour
	uc.redis.SetTokenBlack(ctx, claims.ID, refreshTTL)

	// 3. 删除 Session
	uc.redis.DelSession(ctx, claims.SessionID)

	uc.log.Infof("User logged out: sessionID=%s", claims.SessionID)
	return true, "logout success", nil
}

// ========================================
// 7. GetUser 获取用户信息
// ========================================

func (uc *UserUsecase) GetUser(ctx context.Context, id string) (*User, error) {
	user, err := uc.repo.FindByID(ctx, id)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// ========================================
// 8. UpdateUser 更新用户信息（昵称、头像）
// ========================================

func (uc *UserUsecase) UpdateUser(ctx context.Context, id, nickname, avatar string) (*User, error) {
	user, err := uc.repo.FindByID(ctx, id)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}

	if nickname != "" {
		user.Nickname = nickname
	}
	if avatar != "" {
		user.Avatar = avatar
	}
	user.UpdateTime = time.Now().Format(time.RFC3339)

	return uc.repo.Update(ctx, user)
}

// ========================================
// 9. UpdateProfile 完善用户信息（邮箱）
// ========================================

func (uc *UserUsecase) UpdateProfile(ctx context.Context, id, email string) (*User, error) {
	user, err := uc.repo.FindByID(ctx, id)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}

	// 邮箱唯一性校验
	if email != "" && email != user.Email {
		// 检查邮箱是否已被其他用户使用
		existing, err := uc.repo.FindByEmail(ctx, email)
		if err == nil && existing != nil {
			return nil, ErrEmailAlreadyUsed
		}
		user.Email = email
	}
	user.UpdateTime = time.Now().Format(time.RFC3339)

	return uc.repo.Update(ctx, user)
}

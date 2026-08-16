package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	passportaccessctl "github.com/liuhengloveyou/passport/v3/accessctl"
	passportcommon "github.com/liuhengloveyou/passport/v3/common"
	passportdao "github.com/liuhengloveyou/passport/v3/dao"
	passporthttp "github.com/liuhengloveyou/passport/v3/face/http"
	"gopkg.in/yaml.v3"
)

const (
	passportRootUID      = 10000
	passportRootTenantID = 10000
	// admin / 123456（passport SYS_PWD + sha256）
	passportRootPasswordHash = "39bc31cf8058173e227eadb3af449360f07e95088e0a366f67a04ac361ec559f"
)

var passportMenuPaths = []string{
	"/map", "/preview", "/recording",
	"/review", "/alarms", "/alarms/rules", "/search",
	"/devices",
	"/models", "/algorithms", "/workflows",
	"/notifications",
	"/users", "/system",
}

var passportUserCenterAPIs = []string{
	"user/auth", "user/logout", "user/info", "user/modify", "user/modify/password",
	"access/getRolesForMe", "access/getPolicyForUser",
	"access/getRolesForUser", "access/addRoleForUser", "access/removeRoleForUser",
	"access/getPolicy", "access/addPolicyToRole", "access/removePolicyFromRole",
	"tenant/getUsers", "tenant/user/add", "tenant/delUser", "tenant/userDisableByUID",
	"tenant/getRoles", "tenant/addRole", "tenant/delRole", "tenant/updateConfiguration",
	"tenant/loadConfiguration",
}

// initPassport 初始化 passport 表结构、root 种子、角色策略，并挂载 /usercenter。
// 须在 app.Init 之后、api.Init 之前调用。
func initPassport() {
	if err := bootPassport(); err != nil {
		panic("passport: " + err.Error())
	}
}

func bootPassport() error {
	if err := os.MkdirAll("./data", 0o755); err != nil {
		return fmt.Errorf("mkdir data: %w", err)
	}
	if err := os.MkdirAll("./logs", 0o755); err != nil {
		return fmt.Errorf("mkdir logs: %w", err)
	}

	// passport/common.init 在 flag.Parse 前按默认路径加载；此处按 -passport / 环境变量重载。
	if err := reloadPassportConfig(); err != nil {
		return err
	}

	cfg := &passportcommon.ServConfig
	if strings.TrimSpace(cfg.SessionKey) == "" {
		return fmt.Errorf("passport 未加载配置：请在工作目录放置 passport.conf.yaml，或启动时指定 -passport / 设置 PASSPORT_CONFIG")
	}
	if cfg.DBDriver == "" || cfg.DBDSN == "" {
		return fmt.Errorf("passport 缺少 db_driver / db_dsn")
	}
	if cfg.RootTenantID == 0 {
		cfg.RootTenantID = passportRootTenantID
	}

	if err := passportdao.Init(cfg); err != nil {
		return fmt.Errorf("passport dao.Init: %w", err)
	}

	if err := ensurePassportRootAdminAndTenant(); err != nil {
		return fmt.Errorf("seed root: %w", err)
	}

	// 挂载 /usercenter；内部 InitAccessControl 固定读 cwd 下 rbac_with_domains_model.conf
	_ = findPassportRBACModel()
	passporthttp.InitAndRunHttpApi(nil)

	if err := passportaccessctl.AddRoleForUserInDomain(passportRootUID, passportRootTenantID, "root"); err != nil {
		return fmt.Errorf("AddRoleForUserInDomain root: %w", err)
	}
	if err := seedPassportRolePolicies(); err != nil {
		return fmt.Errorf("seed policies: %w", err)
	}
	return nil
}

func reloadPassportConfig() error {
	path := resolvePassportConfigPath()
	if path == "" {
		return fmt.Errorf("passport 未加载配置：请在工作目录放置 passport.conf.yaml，或启动时指定 -passport / 设置 PASSPORT_CONFIG")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 passport 配置 %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &passportcommon.ServConfig); err != nil {
		return fmt.Errorf("解析 passport 配置 %s: %w", path, err)
	}
	if err := passportcommon.InitWithOption(&passportcommon.ServConfig); err != nil {
		return fmt.Errorf("passport InitWithOption: %w", err)
	}
	return nil
}

func resolvePassportConfigPath() string {
	if v := strings.TrimSpace(os.Getenv("PASSPORT_CONFIG")); v != "" {
		if st, err := os.Stat(v); err == nil && !st.IsDir() {
			return v
		}
	}
	if v := passportFlagFromArgs(); v != "" {
		if st, err := os.Stat(v); err == nil && !st.IsDir() {
			return v
		}
	}
	for _, c := range []string{
		"./passport.conf.yaml",
		"configs/passport.conf.yaml",
	} {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

func passportFlagFromArgs() string {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-passport" || a == "--passport":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(a, "-passport="):
			return strings.TrimPrefix(a, "-passport=")
		case strings.HasPrefix(a, "--passport="):
			return strings.TrimPrefix(a, "--passport=")
		}
	}
	return ""
}

func findPassportRBACModel() string {
	candidates := []string{
		"rbac_with_domains_model.conf",
		"configs/rbac_with_domains_model.conf",
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return "rbac_with_domains_model.conf"
}

func ensurePassportRootAdminAndTenant() error {
	db := passportcommon.DB
	if db == nil {
		return fmt.Errorf("passport DB 未初始化")
	}
	ctx := context.Background()

	rolesJSON := `{"roles":[{"title":"超级管理员","value":"root"},{"title":"管理员","value":"admin"},{"title":"运维","value":"operator"},{"title":"只读","value":"viewer"}]}`

	userSQL := `
INSERT INTO users (
  uid, tenant_id, nickname, cellphone, email, password, ext, create_time, update_time
) VALUES (
  ?, ?, 'admin', '13800000000', 'admin@frameflow.local',
  ?, '{"disabled":0}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
)
ON CONFLICT(uid) DO UPDATE SET
  tenant_id = excluded.tenant_id,
  nickname = excluded.nickname,
  cellphone = excluded.cellphone,
  email = excluded.email,
  password = excluded.password,
  update_time = CURRENT_TIMESTAMP
`
	if _, err := db.Exec(ctx, userSQL, passportRootUID, passportRootTenantID, passportRootPasswordHash); err != nil {
		if _, err2 := db.Exec(ctx, `
INSERT OR IGNORE INTO users (
  uid, tenant_id, nickname, cellphone, email, password, ext, create_time, update_time
) VALUES (?, ?, 'admin', '13800000000', 'admin@frameflow.local', ?, '{"disabled":0}', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
`, passportRootUID, passportRootTenantID, passportRootPasswordHash); err2 != nil {
			return fmt.Errorf("insert user: %v / %v", err, err2)
		}
	}

	tenantSQL := `
INSERT INTO tenants (
  id, uid, tenant_name, tenant_type, info, configuration, create_time, update_time
) VALUES (
  ?, ?, '系统超级管理员', 'admin', '{}', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
)
ON CONFLICT(id) DO UPDATE SET
  uid = excluded.uid,
  tenant_name = excluded.tenant_name,
  configuration = excluded.configuration,
  update_time = CURRENT_TIMESTAMP
`
	if _, err := db.Exec(ctx, tenantSQL, passportRootTenantID, passportRootUID, rolesJSON); err != nil {
		if _, err2 := db.Exec(ctx, `
INSERT OR IGNORE INTO tenants (
  id, uid, tenant_name, tenant_type, info, configuration, create_time, update_time
) VALUES (?, ?, '系统超级管理员', 'admin', '{}', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
`, passportRootTenantID, passportRootUID, rolesJSON); err2 != nil {
			return fmt.Errorf("insert tenant: %v / %v", err, err2)
		}
		_, _ = db.Exec(ctx, `UPDATE tenants SET configuration = ?, update_time = CURRENT_TIMESTAMP WHERE id = ?`, rolesJSON, passportRootTenantID)
	}

	_, _ = db.Exec(ctx, `
INSERT OR IGNORE INTO user_closure (ancestor_id, descendant_id, depth) VALUES (?, ?, 0)
`, passportRootUID, passportRootUID)
	_, _ = db.Exec(ctx, `
INSERT OR IGNORE INTO tenant_closure (ancestor_id, descendant_id, depth) VALUES (?, ?, 0)
`, passportRootTenantID, passportRootTenantID)

	return nil
}

func seedPassportRolePolicies() error {
	for _, obj := range passportMenuPaths {
		if err := passportaccessctl.AddPolicyToRole(passportRootTenantID, "admin", obj, "GET"); err != nil {
			return err
		}
	}
	for _, obj := range passportUserCenterAPIs {
		for _, act := range []string{"GET", "POST", "PUT", "DELETE"} {
			if err := passportaccessctl.AddPolicyToRole(passportRootTenantID, "admin", obj, act); err != nil {
				return err
			}
		}
	}

	opMenus := []string{
		"/map", "/preview", "/devices", "/models", "/algorithms",
		"/workflows", "/alarms", "/system",
	}
	for _, obj := range opMenus {
		if err := passportaccessctl.AddPolicyToRole(passportRootTenantID, "operator", obj, "GET"); err != nil {
			return err
		}
	}
	for _, obj := range []string{"user/auth", "user/logout", "user/info", "access/getRolesForMe", "access/getPolicyForUser"} {
		for _, act := range []string{"GET", "POST"} {
			_ = passportaccessctl.AddPolicyToRole(passportRootTenantID, "operator", obj, act)
		}
	}

	for _, obj := range []string{"/map", "/preview", "/devices", "/alarms"} {
		if err := passportaccessctl.AddPolicyToRole(passportRootTenantID, "viewer", obj, "GET"); err != nil {
			return err
		}
	}
	for _, obj := range []string{"user/auth", "user/logout", "user/info", "access/getRolesForMe", "access/getPolicyForUser"} {
		for _, act := range []string{"GET", "POST"} {
			_ = passportaccessctl.AddPolicyToRole(passportRootTenantID, "viewer", obj, act)
		}
	}
	return nil
}

CREATE TABLE IF NOT EXISTS guorai_credentials (
    credential_key TEXT PRIMARY KEY,
    username TEXT NOT NULL CHECK (BTRIM(username) <> ''),
    password_value TEXT NOT NULL CHECK (password_value <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (credential_key = 'default')
);

REVOKE ALL ON TABLE guorai_credentials FROM PUBLIC;

COMMENT ON TABLE guorai_credentials IS '薯量自动登录凭据，由数据库权限控制访问';
COMMENT ON COLUMN guorai_credentials.credential_key IS '凭据槽位，当前固定为 default';
COMMENT ON COLUMN guorai_credentials.username IS '薯量登录账号';
COMMENT ON COLUMN guorai_credentials.password_value IS '薯量登录密码，按原值保存';
COMMENT ON COLUMN guorai_credentials.created_at IS '凭据首次保存时间';
COMMENT ON COLUMN guorai_credentials.updated_at IS '凭据最近更新时间';

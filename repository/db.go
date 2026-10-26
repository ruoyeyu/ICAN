package repository

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/libtnb/sqlite"
	"github.com/tigerowo/infinite-canvas/config"
	"github.com/tigerowo/infinite-canvas/model"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var promptCategories = []model.PromptCategory{{Category: "system", Name: "个人提示词", Description: "用户自行维护的提示词"}}

var (
	db     *gorm.DB
	dbOnce sync.Once
	dbErr  error
)

// DB 初始化并返回全局数据库连接。
func DB() (*gorm.DB, error) {
	dbOnce.Do(func() {
		driver := strings.ToLower(strings.TrimSpace(config.Cfg.StorageDriver))
		if driver == "" {
			driver = "sqlite"
		}
		dsn := config.Cfg.DatabaseDSN
		if driver == "sqlite" && dsn != ":memory:" {
			_ = os.MkdirAll(filepath.Dir(dsn), 0700)
		}
		if driver == "sqlite" {
			path, query, _ := strings.Cut(dsn, "?")
			params, err := url.ParseQuery(query)
			if err != nil {
				dbErr = err
				return
			}
			params.Add("_pragma", "journal_mode(WAL)")
			params.Set("_txlock", "immediate")
			dsn = path + "?" + params.Encode()
		}
		if isPostgresDriver(driver) {
			dbErr = ensurePostgresDatabase(dsn)
			if dbErr != nil {
				return
			}
		}
		if driver == "mysql" {
			dbErr = ensureMySQLDatabase(dsn)
			if dbErr != nil {
				return
			}
		}
		db, dbErr = gorm.Open(dialector(driver, dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		if dbErr != nil {
			return
		}
		dbErr = db.AutoMigrate(
			&model.LoginSession{},
			&model.User{},
			&model.CreditLog{},
			&model.Prompt{},
			&model.AgentSkill{},
			&model.AgentSkillFile{},
			&model.Asset{},
			&model.Setting{},
			&model.CreativeWorkflow{},
			&model.UserConfig{},
			&model.AICallLog{},
			&model.StorageObject{},
			&model.VideoTask{},
			&model.VideoGenerationLog{},
			&model.ImageGenerationLog{},
			&model.CanvasImageTask{},
			&model.CanvasAudioTask{},
			&model.CanvasProject{},
			&model.ComfyBridge{},
			&model.ComfyBridgeRequest{},
		)
		if dbErr == nil {
			dbErr = migrateInvitationsV1(db)
		}
		if dbErr == nil {
			dbErr = migrateVideoRetrievalV1(db)
		}
	})
	return db, dbErr
}

func dialector(driver string, dsn string) gorm.Dialector {
	switch driver {
	case "mysql":
		return gormmysql.Open(dsn)
	case "postgres", "postgresql":
		return postgres.Open(dsn)
	default:
		return sqlite.Open(dsn)
	}
}

func isPostgresDriver(driver string) bool {
	return driver == "postgres" || driver == "postgresql"
}

func ensureMySQLDatabase(dsn string) error {
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return err
	}
	target := strings.TrimSpace(cfg.DBName)
	if target == "" {
		return nil
	}
	ctx := context.Background()
	targetDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	err = targetDB.PingContext(ctx)
	_ = targetDB.Close()
	if err == nil {
		return nil
	}
	if !isMySQLError(err, 1049) {
		return err
	}

	maintenance := cfg.Clone()
	maintenance.DBName = ""
	serverDB, err := sql.Open("mysql", maintenance.FormatDSN())
	if err != nil {
		return err
	}
	defer serverDB.Close()

	_, err = serverDB.ExecContext(ctx, "CREATE DATABASE "+quoteMySQLIdentifier(target)+" CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci")
	if isMySQLError(err, 1007) {
		return nil
	}
	return err
}

func ensurePostgresDatabase(dsn string) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return err
	}
	target := strings.TrimSpace(cfg.Database)
	if target == "" {
		return nil
	}
	ctx := context.Background()
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err == nil {
		_ = conn.Close(ctx)
		return nil
	}
	if !isPostgresError(err, "3D000") {
		return err
	}

	maintenance := cfg.Copy()
	maintenance.Database = "postgres"
	if strings.EqualFold(target, "postgres") {
		maintenance.Database = "template1"
	}
	conn, err = pgx.ConnectConfig(ctx, maintenance)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{target}.Sanitize(), pgx.QueryExecModeExec)
	if isPostgresError(err, "42P04") {
		return nil
	}
	return err
}

func isMySQLError(err error, number uint16) bool {
	var mysqlErr *mysqldriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == number
}

func isPostgresError(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}

func quoteMySQLIdentifier(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

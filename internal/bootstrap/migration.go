package bootstrap

import (
	"log/slog"

	"agent-desk/internal/migration"
	"agent-desk/internal/models"

	"github.com/mlogclub/simple/sqls"
)

func InitMigrations() error {
	if err := sqls.DB().AutoMigrate(models.Models...); err != nil {
		return err
	}
	if err := migration.Migrate(); err != nil {
		return err
	}
	if _, err := SeedCSSessionsIfEmpty(); err != nil {
		slog.Warn("auto seed cs sessions failed", "err", err)
	}
	return nil
}

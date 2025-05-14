package cron_config

type Config struct {
	// Heartbeat check, every minute
	CronScheduleHeartbeat           string `env:"CRON_SCHEDULE_HEARTBEAT" envDefault:"0 * * * * *"`
	CronScheduleCheckCNAME          string `env:"CRON_SCHEDULE_CHECK_CNAME" envDefault:"0 */5 * * * *"`
	CronScheduleProcessOutboxEvents string `env:"CRON_SCHEDULE_PROCESS_OUTBOX_EVENTS" envDefault:"0 */1 * * * *"`
	CronScheduleProcessWebSessions  string `env:"CRON_PROCESS_WEB_SESSIONS" envDefault:"0 */2 * * * *"`
	CronScheduleOutboxCleanup       string `env:"CRON_SCHEDULE_OUTBOX_CLEANUP" envDefault:"0 0 * * * *"`
}

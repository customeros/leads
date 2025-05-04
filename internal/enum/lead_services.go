package enum

type LeadsService string

const (
	ProxyManager      LeadsService = "leads.proxy_manager"
	SessionManager    LeadsService = "leads.session_manager"
	WebEventProcessor LeadsService = "leads.web_event_processor"
	WebtrackerService LeadsService = "leads.webtracker_service"
)

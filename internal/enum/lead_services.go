package enum

type LeadsService string

const (
	ICPService        LeadsService = "leads.icp_service"
	ProxyManager      LeadsService = "leads.proxy_manager"
	ScraperService    LeadsService = "leads.scraper_service"
	SessionManager    LeadsService = "leads.session_manager"
	WebEventProcessor LeadsService = "leads.web_event_processor"
	WebtrackerService LeadsService = "leads.webtracker_service"
)

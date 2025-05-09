package enum

type Events string

const (
	EventWebtrackerCreated  Events = "webtracker.created"
	EventWebtrackerUpdated  Events = "webtracker.updated"
	EventWebtrackerArchived Events = "webtracker.archived"

	EventWebtrackerSessionCreated  Events = "webtracker.session.created"
	EventWebtrackerSessionClosed   Events = "webtracker.session.closed"
	EventWebtrackerSessionAnalyzed Events = "webtracker.session.analyzed"

	EventWebtrackerVisitorIdentified Events = "webtracker.visitor.identified"

	EventWebtrackerPageView Events = "webtracker.event.page_viewed"
	EventWebtrackerPageExit Events = "webtracker.event.page_exited"
	EventWebtrackerClick    Events = "webtracker.event.clicked"

	EventProxyWebtrackerCnameConfigured    Events = "proxy.webtracker.cname.configured"
	EventProxyWebtrackerCnameNotConfigured Events = "proxy.webtracker.cname.not_configured"
	EventProxyWebtrackerActivated          Events = "proxy.webtracker.activated"
	EventProxyWebtrackerDeactivated        Events = "proxy.webtracker.deactivated"

	EventLeadCreated                  Events = "lead.created" // TODO switch to identified
	EventLeadStageUpdate              Events = "lead.update.stage"
	EventLeadInitialTargetListCreated Events = "lead.initial_target_list.created"
	EventLeadError                    Events = "lead.error"

	EventAskIPData   Events = "request.verify_ipaddress.ipdata"
	EventAskSnitcher Events = "request.identify_ipaddress.snitcher"

	EventRequestICPProfile Events = "request.icp_profile"
	EventICPProfileCreated Events = "icp_profile.created"

	EventWebsiteCrawled Events = "website.crawled"

	EventWebpageScraped  Events = "webpage.scraped"
	EventWebpageProfiled Events = "webpage.profiled"
)

func (e Events) String() string {
	return string(e)
}

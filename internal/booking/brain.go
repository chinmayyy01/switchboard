package booking

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chinmay/switchboard/internal/agent"
	"github.com/chinmay/switchboard/internal/ai"
	"github.com/chinmay/switchboard/internal/jobs"
)

type Answerer interface {
	Answer(ctx context.Context, tenant, query string, k int, lang string) (ai.AnswerResponse, error)
}

var (
	emergencyWords  = []string{"gas", "flood", "burst", "sewage", "carbon monoxide", "electrical", "sparking", "burning", "no heat"}
	humanWords      = []string{"human", "representative", "real person", "talk to someone", "speak to someone", "manager", "supervisor"}
	cancelWords     = []string{"cancel"}
	rescheduleWords = []string{"reschedule", "change my appointment", "change the appointment", "move my appointment", "move the booking", "change my booking"}
	yesWords        = []string{"yes", "yeah", "yep", "sure", "correct", "confirm", "go ahead", "that works", "please do", "ok", "okay"}
	noWords         = []string{"no", "nope", "not", "don't", "do not", "different", "another"}
	closingWords    = []string{"thank you", "thanks", "bye", "goodbye", "that's all", "that is all", "no thanks"}
	faqWords        = []string{"hours", "open", "close", "closed", "price", "pricing", "cost", "charge", "fee", "how much", "serve", "service area", "warranty", "guarantee", "policy", "refund", "insurance", "cover", "district", "discount", "senior", "payment", "cash", "card"}
)

var serviceWords = []struct {
	name  string
	words []string
}{
	{"leak repair", []string{"leak", "burst pipe", "pipe"}},
	{"drain cleaning", []string{"drain", "clog", "blocked"}},
	{"water heater", []string{"water heater", "hot water"}},
	{"ac service", []string{"ac", "air conditioning", "air conditioner", "cooling"}},
	{"heating", []string{"heating", "furnace", "boiler", "radiator"}},
	{"maintenance", []string{"maintenance", "tune up", "tuneup", "checkup"}},
	{"plumbing repair", []string{"toilet", "sink", "faucet", "shower"}},
	{"general repair", []string{"plumber", "plumbing", "repair"}},
}

var (
	isoRe      = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
	questionRe = regexp.MustCompile(`^(what|where|when|who|why|how|do|does|can|could|is|are|will)\b`)
)

type Brain struct {
	store        *jobs.Store
	kb           Answerer
	tenant       string
	businessName string
	state        string
	service      string
	address      string
	date         string
	window       string
	phone        string
	jobID        string
	offered      []string
	phoneTries   int
}

func New(store *jobs.Store, kb Answerer, tenant, businessName string) *Brain {
	return &Brain{store: store, kb: kb, tenant: tenant, businessName: businessName, state: "greet"}
}

func (b *Brain) name() string {
	if b.businessName != "" {
		return b.businessName
	}
	return "the office"
}

func (b *Brain) Handle(ctx context.Context, turn agent.Turn) (agent.Outcome, error) {
	text := strings.TrimSpace(turn.UserText)
	low := strings.ToLower(text)

	switch {
	case hasAny(low, emergencyWords):
		return b.escalate("This sounds like an emergency. I'm connecting you to a person now.", "emergency"), nil
	case hasAny(low, humanWords):
		return b.escalate("Of course, connecting you to a person.", "caller_requested_human"), nil
	case hasAny(low, faqWords):
		return b.answerFAQ(ctx, text, turn.Lang)
	}

	switch b.state {
	case "booked", "done", "cancelled", "rescheduled", "kept":
		if isClosing(low) {
			return b.out("You're all set. Goodbye!", "closed", nil, true, "hangup"), nil
		}
		return b.out("You're all set. Is there anything else?", b.state, nil, false, ""), nil
	case "escalate":
		return b.out("A person will be with you shortly.", "escalate", nil, true, "escalated"), nil
	}

	switch b.state {
	case "", "greet":
		return b.greet(low, text)
	case "service":
		return b.serviceStep(low, text)
	case "address":
		return b.addressStep(low, text)
	case "time", "offer":
		return b.timeStep(low, text)
	case "phone":
		return b.phoneStep(low, text)
	case "confirm":
		return b.confirmStep(low)
	case "cancel_phone":
		return b.cancelPhone(low)
	case "cancel_confirm":
		return b.cancelConfirm(low)
	case "resched_phone":
		return b.reschedPhone(low)
	case "resched_time":
		return b.reschedTime(low)
	case "resched_confirm":
		return b.reschedConfirm(low)
	}
	return b.escalate("Sorry, let me connect you to a person.", "policy"), nil
}

func (b *Brain) greet(low, text string) (agent.Outcome, error) {
	switch {
	case hasAny(low, cancelWords):
		return b.out("I can cancel that. What's the phone number on the booking?", "cancel_phone", nil, false, ""), nil
	case hasAny(low, rescheduleWords):
		return b.out("I can reschedule that. What's the phone number on the booking?", "resched_phone", nil, false, ""), nil
	}
	if svc := classifyService(low); svc != "" {
		b.service = svc
		return b.out("Got it, "+svc+". What's the service address?", "address", nil, false, ""), nil
	}
	return b.out("Thanks for calling "+b.name()+". What do you need help with today?", "service", nil, false, ""), nil
}

func (b *Brain) serviceStep(low, text string) (agent.Outcome, error) {
	svc := classifyService(low)
	if svc == "" {
		svc = clean(text)
	}
	if svc == "" || isQuestion(low) {
		return b.out("What do you need help with: a leak, drain, water heater, AC, or heating?", "service", nil, false, ""), nil
	}
	b.service = svc
	return b.out("Got it, "+svc+". What's the service address?", "address", nil, false, ""), nil
}

func (b *Brain) addressStep(low, text string) (agent.Outcome, error) {
	addr := cleanAddress(text)
	if addr == "" || isQuestion(low) || !validAddress(addr) {
		return b.out("Sorry, I didn't catch the address. Could you repeat it?", "address", nil, false, ""), nil
	}
	b.address = addr
	return b.out("What day and window works? We have morning, afternoon, or evening.", "time", nil, false, ""), nil
}

func (b *Brain) timeStep(low, text string) (agent.Outcome, error) {
	if b.state == "offer" {
		w := parseWindow(low)
		if w == "" {
			return b.out("Which window: morning, afternoon, or evening?", "offer", nil, false, ""), nil
		}
		if len(b.offered) > 0 && !contains(b.offered, w) {
			return b.out("I have "+joinWindows(b.offered)+". Which works?", "offer", nil, false, ""), nil
		}
		b.window = w
		return b.out("Great. What's the best callback number?", "phone", nil, false, ""), nil
	}

	d := parseDate(low, b.store.Now())
	w := parseWindow(low)
	if d != "" {
		b.date = d
	}
	if w != "" {
		b.window = w
	}
	if b.date == "" {
		return b.out("Which day works for you?", "time", nil, false, ""), nil
	}

	avail := b.store.Availability(b.date)
	if len(avail) == 0 {
		b.date = ""
		return b.out("That day is fully booked. What other day works?", "time", nil, false, ""), nil
	}
	if b.window == "" {
		b.offered = avail
		return b.out("On "+prettyDate(b.date)+" I have "+joinWindows(avail)+". Which works?", "offer", toolCall("check_availability", map[string]any{"date": b.date}), false, ""), nil
	}
	if !contains(avail, b.window) {
		b.window = ""
		b.offered = avail
		return b.out("That window is full. I have "+joinWindows(avail)+" on "+prettyDate(b.date)+". Which works?", "offer", toolCall("check_availability", map[string]any{"date": b.date}), false, ""), nil
	}
	return b.out("Great. What's the best callback number?", "phone", nil, false, ""), nil
}

func (b *Brain) phoneStep(_ string, text string) (agent.Outcome, error) {
	p := validPhone(digitsFromSpeech(text))
	if p == "" {
		return b.out("I need a 10-digit callback number, including the area code. Could you say it again?", "phone", nil, false, ""), nil
	}
	b.phone = p
	args := map[string]any{"service": b.service, "address": b.address, "date": b.date, "window": b.window}
	return b.out(
		fmt.Sprintf("To confirm: %s at %s on %s, %s window, callback %s. Shall I book it?", b.service, b.address, prettyDate(b.date), b.window, speakPhone(b.phone)),
		"confirm", toolCall("check_availability", args), false, "",
	), nil
}

func (b *Brain) confirmStep(low string) (agent.Outcome, error) {
	switch {
	case isYes(low):
		j, err := b.store.Book(b.service, b.date, b.window, b.address, b.phone)
		if err != nil {
			b.state = "time"
			return b.out("Sorry, that slot just got taken. What other time works?", "time", nil, false, ""), nil
		}
		b.jobID = j.ID
		return b.out(
			fmt.Sprintf("Booked. %s on %s, %s window at %s. Your reference is %s.", j.Service, prettyDate(j.Date), j.Window, j.Address, j.ID),
			"booked", toolCall("book", map[string]any{"id": j.ID}), true, "booked",
		), nil
	case isNo(low):
		return b.out("No problem. What other day or window works?", "time", nil, false, ""), nil
	}
	return b.out("Shall I book it? Please say yes or no.", "confirm", nil, false, ""), nil
}

func (b *Brain) cancelPhone(low string) (agent.Outcome, error) {
	p := validPhone(digitsFromSpeech(low))
	if p == "" {
		return b.out("What's the 10-digit phone number on the booking?", "cancel_phone", nil, false, ""), nil
	}
	found := b.store.ByPhone(p)
	if len(found) == 0 {
		b.phoneTries++
		if b.phoneTries >= 2 {
			return b.escalate("I can't find that booking. Let me connect you to a person.", "not_found"), nil
		}
		return b.out("I couldn't find a booking for that number. Can you confirm it?", "cancel_phone", nil, false, ""), nil
	}
	j := found[0]
	b.jobID = j.ID
	b.phone = p
	b.service = j.Service
	b.date = j.Date
	b.window = j.Window
	return b.out(
		fmt.Sprintf("I found %s on %s, %s window. Cancel it?", j.Service, prettyDate(j.Date), j.Window),
		"cancel_confirm", toolCall("lookup_job", map[string]any{"phone": p}), false, "",
	), nil
}

func (b *Brain) cancelConfirm(low string) (agent.Outcome, error) {
	switch {
	case isYes(low):
		if err := b.store.Cancel(b.jobID); err != nil {
			return b.escalate("I couldn't cancel that. Let me connect you to a person.", "tool_error"), nil
		}
		return b.out("Cancelled. Anything else?", "done", toolCall("cancel", map[string]any{"id": b.jobID}), true, "cancelled"), nil
	case isNo(low):
		return b.out("Okay, I'll leave it as is. Anything else?", "done", nil, true, "kept"), nil
	}
	return b.out("Just to confirm, should I cancel it? Yes or no.", "cancel_confirm", nil, false, ""), nil
}

func (b *Brain) reschedPhone(low string) (agent.Outcome, error) {
	p := validPhone(digitsFromSpeech(low))
	if p == "" {
		return b.out("What's the 10-digit phone number on the booking?", "resched_phone", nil, false, ""), nil
	}
	found := b.store.ByPhone(p)
	if len(found) == 0 {
		b.phoneTries++
		if b.phoneTries >= 2 {
			return b.escalate("I can't find that booking. Let me connect you to a person.", "not_found"), nil
		}
		return b.out("I couldn't find a booking for that number. Can you confirm it?", "resched_phone", nil, false, ""), nil
	}
	j := found[0]
	b.jobID = j.ID
	b.phone = p
	b.service = j.Service
	b.address = j.Address
	return b.out(
		fmt.Sprintf("I found %s on %s, %s window. What new day and window would you like?", j.Service, prettyDate(j.Date), j.Window),
		"resched_time", toolCall("lookup_job", map[string]any{"phone": p}), false, "",
	), nil
}

func (b *Brain) reschedTime(low string) (agent.Outcome, error) {
	d := parseDate(low, b.store.Now())
	w := parseWindow(low)
	if d != "" {
		b.date = d
	}
	if w != "" {
		b.window = w
	}
	if b.date == "" || b.window == "" {
		return b.out("What new day and window? Morning, afternoon, or evening.", "resched_time", nil, false, ""), nil
	}
	avail := b.store.Availability(b.date)
	if !contains(avail, b.window) {
		b.window = ""
		return b.out("That window is full on "+prettyDate(b.date)+". I have "+joinWindows(avail)+". Which works?", "resched_time", toolCall("check_availability", map[string]any{"date": b.date}), false, ""), nil
	}
	return b.out(
		fmt.Sprintf("Move it to %s, %s window? Shall I confirm?", prettyDate(b.date), b.window),
		"resched_confirm", toolCall("check_availability", map[string]any{"date": b.date}), false, "",
	), nil
}

func (b *Brain) reschedConfirm(low string) (agent.Outcome, error) {
	switch {
	case isYes(low):
		if err := b.store.Reschedule(b.jobID, b.date, b.window); err != nil {
			return b.escalate("I couldn't move that booking. Let me connect you to a person.", "tool_error"), nil
		}
		return b.out("Moved. Anything else?", "done", toolCall("reschedule", map[string]any{"id": b.jobID, "date": b.date, "window": b.window}), true, "rescheduled"), nil
	case isNo(low):
		return b.out("Okay, I'll leave it as is. Anything else?", "done", nil, true, "kept"), nil
	}
	return b.out("Shall I move it? Yes or no.", "resched_confirm", nil, false, ""), nil
}

func (b *Brain) answerFAQ(ctx context.Context, question, lang string) (agent.Outcome, error) {
	if b.kb == nil {
		return b.escalate("I'd rather not guess on that. Let me connect you to a person.", "ungrounded"), nil
	}
	res, err := b.kb.Answer(ctx, b.tenant, question, 3, lang)
	if err != nil || !res.Grounded || res.Answer == "" {
		return b.escalate("I'm not sure about that. Let me connect you to a person.", "ungrounded"), nil
	}
	return agent.Outcome{
		Reply:      res.Answer,
		State:      "faq",
		ToolCalled: toolCall("lookup_policy", map[string]any{"query": question}),
		Citations:  res.Citations,
	}, nil
}

func (b *Brain) escalate(reply, reason string) agent.Outcome {
	b.state = "escalate"
	return agent.Outcome{Reply: reply, State: "escalate", Escalate: true, EndCall: true, EndReason: reason}
}

func (b *Brain) out(reply, state string, tool *agent.ToolCall, end bool, reason string) agent.Outcome {
	if state != "" {
		b.state = state
	}
	return agent.Outcome{Reply: reply, State: b.state, ToolCalled: tool, EndCall: end, EndReason: reason}
}

func classifyService(low string) string {
	for _, entry := range serviceWords {
		for _, w := range entry.words {
			if serviceMatch(low, w) {
				return entry.name
			}
		}
	}
	return ""
}

func serviceMatch(low, w string) bool {
	if w == "ac" {
		return containsWord(low, w)
	}
	return strings.Contains(low, w)
}

var numberWords = map[string]byte{
	"zero": '0', "oh": '0', "one": '1', "two": '2', "to": '2', "too": '2',
	"three": '3', "four": '4', "for": '4', "five": '5', "six": '6',
	"seven": '7', "eight": '8', "ate": '8', "nine": '9',
}

func digitsFromSpeech(text string) string {
	low := strings.ToLower(text)
	hasDigit := false
	for _, r := range low {
		if r >= '0' && r <= '9' {
			hasDigit = true
			break
		}
	}

	var b strings.Builder
	for _, tok := range strings.FieldsFunc(low, func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if tok == "" {
			continue
		}
		isNum := true
		for _, r := range tok {
			if r < '0' || r > '9' {
				isNum = false
				break
			}
		}
		if isNum {
			b.WriteString(tok)
			continue
		}
		if !hasDigit {
			if d, ok := numberWords[tok]; ok {
				b.WriteByte(d)
			}
		}
	}
	return b.String()
}

func validPhone(p string) string {
	if len(p) == 11 && p[0] == '1' {
		p = p[1:]
	}
	if len(p) != 10 {
		return ""
	}
	return p
}

func speakPhone(p string) string {
	if len(p) != 10 {
		return p
	}
	parts := make([]string, 0, 10)
	for _, r := range p {
		parts = append(parts, string(r))
	}
	return strings.Join(parts, " ")
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

var inDaysRe = regexp.MustCompile(`in\s+(\d+)\s+(day|days|week|weeks)`)

func parseInDays(low string, now time.Time) string {
	m := inDaysRe.FindStringSubmatch(low)
	if m == nil {
		return ""
	}
	n := atoi(m[1])
	if strings.HasPrefix(m[2], "week") {
		n *= 7
	}
	return now.AddDate(0, 0, n).Format("2006-01-02")
}

func monthFrom(s string) (time.Month, bool) {
	months := map[string]time.Month{
		"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
		"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
	}
	if len(s) < 3 {
		return 0, false
	}
	m, ok := months[s[:3]]
	return m, ok
}

var monthFirstRe = regexp.MustCompile(`\b(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\s+(\d{1,2})\b`)
var dayFirstRe = regexp.MustCompile(`\b(\d{1,2})\s+(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:tember)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)\b`)

func parseMonthDay(low string, now time.Time) string {
	day := 0
	var mon time.Month

	if m := monthFirstRe.FindStringSubmatch(low); m != nil {
		mm, ok := monthFrom(m[1])
		if !ok {
			return ""
		}
		mon, day = mm, atoi(m[2])
	} else if m := dayFirstRe.FindStringSubmatch(low); m != nil {
		mm, ok := monthFrom(m[2])
		if !ok {
			return ""
		}
		mon, day = mm, atoi(m[1])
	} else {
		return ""
	}

	if day < 1 || day > 31 {
		return ""
	}
	t := time.Date(now.Year(), mon, day, 0, 0, 0, 0, now.Location())
	if t.Day() != day {
		return ""
	}
	if t.Before(dateOnly(now)) {
		t = time.Date(now.Year()+1, mon, day, 0, 0, 0, 0, now.Location())
	}
	return t.Format("2006-01-02")
}

var ordinalRe = regexp.MustCompile(`\b(\d{1,2})(?:st|nd|rd|th)\b`)

func parseOrdinal(low string, now time.Time) string {
	m := ordinalRe.FindStringSubmatch(low)
	if m == nil {
		return ""
	}
	day := atoi(m[1])
	if day < 1 || day > 31 {
		return ""
	}
	for i := 0; i < 12; i++ {
		t := time.Date(now.Year(), now.Month()+time.Month(i), day, 0, 0, 0, 0, now.Location())
		if t.Day() != day {
			continue
		}
		if !t.Before(dateOnly(now)) {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

func parseWeekday(low string, now time.Time) string {
	days := []string{"sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"}
	for i, d := range days {
		if containsWord(low, d) {
			delta := (i - int(now.Weekday()) + 7) % 7
			if delta == 0 {
				delta = 7
			}
			return now.AddDate(0, 0, delta).Format("2006-01-02")
		}
	}
	return ""
}

func parseDate(low string, now time.Time) string {
	if m := isoRe.FindString(low); m != "" {
		if t, err := time.Parse("2006-01-02", m); err == nil && !t.Before(dateOnly(now)) {
			return m
		}
	}
	if strings.Contains(low, "today") {
		return now.Format("2006-01-02")
	}
	if strings.Contains(low, "tomorrow") {
		return now.AddDate(0, 0, 1).Format("2006-01-02")
	}
	if d := parseInDays(low, now); d != "" {
		return d
	}
	if d := parseMonthDay(low, now); d != "" {
		return d
	}
	if d := parseOrdinal(low, now); d != "" {
		return d
	}
	return parseWeekday(low, now)
}

func parseWindow(low string) string {
	switch {
	case strings.Contains(low, "morning"):
		return "morning"
	case strings.Contains(low, "afternoon"):
		return "afternoon"
	case strings.Contains(low, "evening"), strings.Contains(low, "night"):
		return "evening"
	case strings.Contains(low, "pm"):
		return "afternoon"
	case strings.Contains(low, "am"):
		return "morning"
	}
	return ""
}

func prettyDate(iso string) string {
	t, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return t.Format("Mon Jan 2")
}

func joinWindows(ws []string) string {
	switch len(ws) {
	case 0:
		return "no windows"
	case 1:
		return ws[0]
	case 2:
		return ws[0] + " or " + ws[1]
	}
	return strings.Join(ws[:len(ws)-1], ", ") + ", or " + ws[len(ws)-1]
}

func containsWord(low, w string) bool {
	if strings.Contains(w, " ") || strings.Contains(w, "-") {
		return strings.Contains(low, w)
	}
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(w) + `\b`).MatchString(low)
}

func hasAny(low string, words []string) bool {
	for _, w := range words {
		if containsWord(low, w) {
			return true
		}
	}
	return false
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func isYes(low string) bool { return hasAny(low, yesWords) }
func isNo(low string) bool  { return hasAny(low, noWords) }

func isQuestion(low string) bool {
	return strings.HasSuffix(strings.TrimSpace(low), "?") || questionRe.MatchString(low)
}

func clean(text string) string {
	return strings.TrimSpace(strings.Trim(text, " .,!?;:"))
}

func cleanAddress(text string) string {
	s := clean(text)
	low := strings.ToLower(s)
	for _, p := range []string{"my address is", "the address is", "address is", "i live at", "i'm at", "im at", "it's", "its"} {
		if strings.HasPrefix(low, p) {
			s = strings.TrimSpace(s[len(p):])
			low = strings.ToLower(s)
		}
	}
	return clean(s)
}

func validAddress(s string) bool {
	low := strings.ToLower(strings.TrimSpace(s))
	if len(low) < 4 {
		return false
	}
	for _, filler := range []string{"um", "uh", "er", "hmm", "umm"} {
		if low == filler {
			return false
		}
	}
	return true
}

func isClosing(low string) bool { return hasAny(low, closingWords) }

func toolCall(name string, args map[string]any) *agent.ToolCall {
	b, _ := json.Marshal(args)
	return &agent.ToolCall{Name: name, Args: b}
}

package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"badmintonhub/internal/modules/identity"
	"badmintonhub/internal/modules/matches"
	"badmintonhub/internal/modules/notifications"
	"badmintonhub/internal/modules/payments"
	"badmintonhub/internal/modules/players"
	"badmintonhub/internal/modules/venues"
)

type R1Routes struct {
	Identity      *identity.Service
	Players       *players.Service
	Venues        venues.Service
	Matches       matches.Service
	Payments      *payments.Service
	Notifications *notifications.Service
}

func (routes R1Routes) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", routes.register)
	mux.HandleFunc("POST /api/v1/auth/verify-email", routes.verifyEmail)
	mux.HandleFunc("POST /api/v1/auth/resend-verification", routes.resendVerification)
	mux.HandleFunc("POST /api/v1/auth/login", routes.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", routes.refresh)
	mux.Handle("POST /api/v1/auth/logout", routes.auth(http.HandlerFunc(routes.logout)))
	mux.HandleFunc("POST /api/v1/auth/forgot-password", routes.forgotPassword)
	mux.HandleFunc("POST /api/v1/auth/reset-password", routes.resetPassword)

	mux.Handle("GET /api/v1/me/onboarding", routes.auth(http.HandlerFunc(routes.getOnboarding)))
	mux.Handle("PUT /api/v1/me/onboarding", routes.auth(http.HandlerFunc(routes.saveOnboarding)))
	mux.Handle("POST /api/v1/me/onboarding/complete", routes.auth(http.HandlerFunc(routes.completeOnboarding)))
	mux.HandleFunc("GET /api/v1/players/{playerID}", routes.publicPlayer)

	mux.Handle("POST /api/v1/organizer-applications", routes.auth(http.HandlerFunc(routes.applyOrganizer)))
	mux.Handle("POST /api/v1/admin/organizer-applications/{applicationID}/review", routes.auth(http.HandlerFunc(routes.reviewOrganizer)))
	mux.Handle("POST /api/v1/admin/organizers/{accountID}/revoke", routes.auth(http.HandlerFunc(routes.revokeOrganizer)))

	mux.HandleFunc("GET /api/v1/venues", routes.listVenues)
	mux.HandleFunc("GET /api/v1/venues/{venueID}", routes.venueDetail)
	mux.Handle("POST /api/v1/venues", routes.auth(http.HandlerFunc(routes.createVenue)))
	mux.Handle("PUT /api/v1/venues/{venueID}", routes.auth(http.HandlerFunc(routes.updateVenue)))
	mux.Handle("POST /api/v1/venues/{venueID}/publish", routes.auth(http.HandlerFunc(routes.publishVenue)))
	mux.Handle("POST /api/v1/venues/{venueID}/hide", routes.auth(http.HandlerFunc(routes.hideVenue)))
	mux.Handle("PUT /api/v1/venues/{venueID}/managers/{managerID}", routes.auth(http.HandlerFunc(routes.assignVenueManager)))

	mux.HandleFunc("GET /api/v1/matches", routes.listMatches)
	mux.HandleFunc("GET /api/v1/matches/{matchID}", routes.matchDetail)
	mux.Handle("POST /api/v1/matches", routes.auth(http.HandlerFunc(routes.createMatch)))
	mux.Handle("PUT /api/v1/matches/{matchID}", routes.auth(http.HandlerFunc(routes.updateMatch)))
	mux.Handle("POST /api/v1/matches/{matchID}/publish", routes.auth(http.HandlerFunc(routes.publishMatch)))
	mux.Handle("POST /api/v1/matches/{matchID}/join", routes.auth(http.HandlerFunc(routes.joinMatch)))
	mux.Handle("POST /api/v1/matches/{matchID}/participants/{participationID}/approve", routes.auth(http.HandlerFunc(routes.approveParticipation)))
	mux.Handle("POST /api/v1/matches/{matchID}/participants/{participationID}/reject", routes.auth(http.HandlerFunc(routes.rejectParticipation)))
	mux.Handle("POST /api/v1/matches/{matchID}/participants/{participationID}/remove", routes.auth(http.HandlerFunc(routes.removeParticipation)))
	routes.registerR2(mux)
}

func (routes R1Routes) auth(next http.Handler) http.Handler {
	return RequireAuth(AuthenticateFunc(func(ctx context.Context, token string) (Principal, error) {
		account, err := routes.Identity.Authenticate(ctx, token)
		if err != nil {
			return Principal{}, err
		}
		return Principal{AccountID: account.ID}, nil
	}), next)
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenRequest struct {
	Token string `json:"token"`
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

type resetPasswordRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

func (routes R1Routes) register(w http.ResponseWriter, r *http.Request) {
	var body credentialsRequest
	if !DecodeJSON(w, r, &body) {
		return
	}
	account, err := routes.Identity.Register(r.Context(), identity.RegisterRequest{Email: body.Email, Password: body.Password, SourceKey: sourceKey(r)})
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, accountResponse(account))
}

func (routes R1Routes) verifyEmail(w http.ResponseWriter, r *http.Request) {
	var body tokenRequest
	if !DecodeJSON(w, r, &body) {
		return
	}
	account, err := routes.Identity.VerifyEmail(r.Context(), body.Token)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, accountResponse(account))
}

func (routes R1Routes) resendVerification(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	if err := routes.Identity.ResendVerification(r.Context(), body.Email, sourceKey(r)); err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSONStatus(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (routes R1Routes) login(w http.ResponseWriter, r *http.Request) {
	var body credentialsRequest
	if !DecodeJSON(w, r, &body) {
		return
	}
	tokens, err := routes.Identity.Login(r.Context(), identity.LoginRequest{Email: body.Email, Password: body.Password, SourceKey: sourceKey(r)})
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, tokensResponse(tokens))
}

func (routes R1Routes) refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshRequest
	if !DecodeJSON(w, r, &body) {
		return
	}
	tokens, err := routes.Identity.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, tokensResponse(tokens))
}

func (routes R1Routes) logout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if err := routes.Identity.Logout(r.Context(), token); err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (routes R1Routes) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	if err := routes.Identity.ForgotPassword(r.Context(), body.Email); err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSONStatus(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (routes R1Routes) resetPassword(w http.ResponseWriter, r *http.Request) {
	var body resetPasswordRequest
	if !DecodeJSON(w, r, &body) {
		return
	}
	if err := routes.Identity.ResetPassword(r.Context(), body.Token, body.NewPassword); err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func accountResponse(account identity.Account) map[string]any {
	return map[string]any{"id": account.ID, "email": account.Email, "state": account.State, "emailVerifiedAt": account.EmailVerifiedAt}
}

func tokensResponse(tokens identity.Tokens) map[string]any {
	return map[string]any{"accessToken": tokens.AccessToken, "accessExpiresAt": tokens.AccessExpiresAt, "refreshToken": tokens.RefreshToken, "refreshExpiresAt": tokens.RefreshExpiresAt}
}

func sourceKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(host)))
	return hex.EncodeToString(sum[:])
}

func bearerToken(r *http.Request) string {
	_, token, _ := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	return token
}

func idempotencyKey(r *http.Request) (string, bool) {
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	return key, key != "" && len(key) <= 128
}

type onboardingRequest struct {
	DisplayName      *string               `json:"displayName"`
	AvatarURL        *string               `json:"avatarUrl"`
	DateOfBirth      *string               `json:"dateOfBirth"`
	Gender           *players.Gender       `json:"gender"`
	Experience       *players.Experience   `json:"experience"`
	SkillLevel       *players.SkillLevel   `json:"skillLevel"`
	PreferredFormats []players.GameFormat  `json:"preferredFormats"`
	PlayStyles       []players.PlayStyle   `json:"playStyles"`
	UsualPeriods     []players.UsualPeriod `json:"usualPeriods"`
	RegularArea      *string               `json:"regularArea"`
}

func (routes R1Routes) getOnboarding(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	profile, err := routes.Players.GetOwnerProfile(r.Context(), principal.AccountID)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, ownerProfileResponse(profile))
}

func (routes R1Routes) saveOnboarding(w http.ResponseWriter, r *http.Request) {
	var body onboardingRequest
	if !DecodeJSON(w, r, &body) {
		return
	}
	draft, err := body.draft()
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	profile, err := routes.Players.SaveDraft(r.Context(), principal.AccountID, draft)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, ownerProfileResponse(profile))
}

func (routes R1Routes) completeOnboarding(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	profile, err := routes.Players.CompleteOnboarding(r.Context(), principal.AccountID)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, ownerProfileResponse(profile))
}

func (routes R1Routes) publicPlayer(w http.ResponseWriter, r *http.Request) {
	profile, err := routes.Players.GetPublicProfile(r.Context(), r.PathValue("playerID"))
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, map[string]any{"accountId": profile.AccountID, "displayName": profile.DisplayName, "skillLevel": profile.SkillLevel, "reliability": profile.Reliability, "matchCount": profile.MatchCount})
}

func (body onboardingRequest) draft() (players.Draft, error) {
	var date *players.CalendarDate
	if body.DateOfBirth != nil {
		parsed, err := players.ParseCalendarDate(*body.DateOfBirth)
		if err != nil {
			return players.Draft{}, err
		}
		date = &parsed
	}
	return players.Draft{DisplayName: body.DisplayName, AvatarURL: body.AvatarURL, DateOfBirth: date, Gender: body.Gender, Experience: body.Experience, SkillLevel: body.SkillLevel, PreferredFormats: body.PreferredFormats, PlayStyles: body.PlayStyles, UsualPeriods: body.UsualPeriods, RegularArea: body.RegularArea}, nil
}

func ownerProfileResponse(profile players.OwnerProfile) map[string]any {
	var date any
	if profile.DateOfBirth != nil {
		date = profile.DateOfBirth.String()
	}
	return map[string]any{
		"accountId": profile.AccountID, "status": profile.Status, "displayName": profile.DisplayName,
		"avatarUrl": profile.AvatarURL, "dateOfBirth": date, "gender": profile.Gender,
		"experience": profile.Experience, "skillLevel": profile.SkillLevel,
		"preferredFormats": profile.PreferredFormats, "playStyles": profile.PlayStyles,
		"usualPeriods": profile.UsualPeriods, "regularArea": profile.RegularArea,
		"skillConfidence": profile.SkillConfidence, "reliability": profile.Reliability,
		"matchCount": profile.MatchCount, "completedAt": profile.CompletedAt, "updatedAt": profile.UpdatedAt,
	}
}

func (routes R1Routes) applyOrganizer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason  string `json:"reason"`
		Contact string `json:"contact"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	application, err := routes.Identity.ApplyOrganizer(r.Context(), identity.OrganizerRequest{AccountID: principal.AccountID, Reason: body.Reason, Contact: body.Contact})
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, organizerResponse(application))
}

func (routes R1Routes) reviewOrganizer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Decision  string `json:"decision"`
		Rationale string `json:"rationale"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	err := routes.Identity.ReviewOrganizer(r.Context(), identity.ReviewOrganizerRequest{ApplicationID: r.PathValue("applicationID"), AdminID: principal.AccountID, Decision: body.Decision, Rationale: body.Rationale})
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (routes R1Routes) revokeOrganizer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Rationale string `json:"rationale"`
	}
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	err := routes.Identity.RevokeOrganizer(r.Context(), identity.RevokeOrganizerRequest{AccountID: r.PathValue("accountID"), AdminID: principal.AccountID, Rationale: body.Rationale})
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func organizerResponse(application identity.OrganizerApplication) map[string]any {
	return map[string]any{"id": application.ID, "accountId": application.AccountID, "reason": application.Reason, "contact": application.Contact, "status": application.Status, "submittedAt": application.SubmittedAt, "reviewedBy": application.ReviewedBy, "reviewedAt": application.ReviewedAt, "reviewRationale": application.ReviewRationale}
}

type venueDraftRequest struct {
	Name         string             `json:"name"`
	Address      string             `json:"address"`
	Area         string             `json:"area"`
	TimeZone     string             `json:"timeZone"`
	Latitude     *float64           `json:"latitude"`
	Longitude    *float64           `json:"longitude"`
	OpeningHours []string           `json:"openingHours"`
	Amenities    []string           `json:"amenities"`
	Photos       []string           `json:"photos"`
	Courts       []string           `json:"courts"`
	Price        *venuePriceRequest `json:"price"`
}

type venuePriceRequest struct {
	AmountMinor     int64     `json:"amountMinor"`
	Currency        string    `json:"currency"`
	Unit            string    `json:"unit"`
	SourceKind      string    `json:"sourceKind"`
	SourceReference string    `json:"sourceReference"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (body venueDraftRequest) draft() venues.Draft {
	var price *venues.PriceGuidance
	if body.Price != nil {
		price = &venues.PriceGuidance{AmountMinor: body.Price.AmountMinor, Currency: body.Price.Currency, Unit: body.Price.Unit, Source: venues.Source{Kind: body.Price.SourceKind, Reference: body.Price.SourceReference, UpdatedAt: body.Price.UpdatedAt}}
	}
	return venues.Draft{Name: body.Name, Address: body.Address, Area: body.Area, TimeZone: body.TimeZone, Latitude: body.Latitude, Longitude: body.Longitude, OpeningHours: body.OpeningHours, Amenities: body.Amenities, Photos: body.Photos, Courts: body.Courts, Price: price}
}

func (routes R1Routes) createVenue(w http.ResponseWriter, r *http.Request) {
	var body venueDraftRequest
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	venue, err := routes.Venues.Create(r.Context(), principal.AccountID, body.draft())
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, venueResponse(venue))
}

func (routes R1Routes) updateVenue(w http.ResponseWriter, r *http.Request) {
	var body venueDraftRequest
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	venue, err := routes.Venues.Update(r.Context(), principal.AccountID, r.PathValue("venueID"), body.draft())
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, venueResponse(venue))
}

func (routes R1Routes) publishVenue(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	venue, err := routes.Venues.Publish(r.Context(), principal.AccountID, r.PathValue("venueID"))
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, venueResponse(venue))
}

func (routes R1Routes) hideVenue(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	venue, err := routes.Venues.Hide(r.Context(), principal.AccountID, r.PathValue("venueID"))
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, venueResponse(venue))
}

func (routes R1Routes) assignVenueManager(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	if err := routes.Venues.AssignManager(r.Context(), principal.AccountID, r.PathValue("venueID"), r.PathValue("managerID")); err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (routes R1Routes) listVenues(w http.ResponseWriter, r *http.Request) {
	limit, ok := queryInt(r, "limit", 20)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "invalid_filter", "A query filter is invalid.")
		return
	}
	var maxPrice *int64
	if raw := r.URL.Query().Get("maxPriceMinor"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			WriteError(w, r, http.StatusBadRequest, "invalid_filter", "A query filter is invalid.")
			return
		}
		maxPrice = &value
	}
	page, err := routes.Venues.List(r.Context(), venues.Filters{Area: r.URL.Query().Get("area"), MaxPriceMinor: maxPrice, Limit: limit, Cursor: r.URL.Query().Get("cursor")})
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(page.Venues))
	for _, venue := range page.Venues {
		items = append(items, venueResponse(venue))
	}
	writeJSON(w, map[string]any{"items": items, "nextCursor": page.NextCursor})
}

func (routes R1Routes) venueDetail(w http.ResponseWriter, r *http.Request) {
	venue, err := routes.Venues.Detail(r.Context(), r.PathValue("venueID"))
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, venueResponse(venue))
}

func venueResponse(venue venues.Venue) map[string]any {
	var price any
	if venue.Price != nil {
		price = map[string]any{"amountMinor": venue.Price.AmountMinor, "currency": venue.Price.Currency, "unit": venue.Price.Unit, "sourceKind": venue.Price.Source.Kind, "sourceReference": venue.Price.Source.Reference, "updatedAt": venue.Price.Source.UpdatedAt}
	}
	return map[string]any{"id": venue.ID, "name": venue.Name, "address": venue.Address, "area": venue.Area, "timeZone": venue.TimeZone, "latitude": venue.Latitude, "longitude": venue.Longitude, "openingHours": venue.OpeningHours, "amenities": venue.Amenities, "photos": venue.Photos, "courts": venue.Courts, "price": price, "status": venue.Status, "updatedAt": venue.UpdatedAt}
}

type matchDraftRequest struct {
	Title                     string           `json:"title"`
	Description               string           `json:"description"`
	Format                    string           `json:"format"`
	Style                     string           `json:"style"`
	Rules                     string           `json:"rules"`
	VenueID                   string           `json:"venueId"`
	Court                     string           `json:"court"`
	StartAt                   time.Time        `json:"startAt"`
	EndAt                     time.Time        `json:"endAt"`
	MinLevel                  int              `json:"minLevel"`
	MaxLevel                  int              `json:"maxLevel"`
	Capacity                  int              `json:"capacity"`
	FeeMinor                  int64            `json:"feeMinor"`
	DepositMinor              int64            `json:"depositMinor"`
	Currency                  string           `json:"currency"`
	PaymentRecipient          string           `json:"paymentRecipient"`
	PaymentInstructions       string           `json:"paymentInstructions"`
	PaymentInstructionVersion int              `json:"paymentInstructionVersion"`
	JoinMode                  matches.JoinMode `json:"joinMode"`
	HostPlays                 bool             `json:"hostPlays"`
	CourtAttested             bool             `json:"courtAttested"`
}

func (body matchDraftRequest) draft() matches.Draft {
	return matches.Draft{Title: body.Title, Description: body.Description, Format: body.Format, Style: body.Style, Rules: body.Rules, VenueID: body.VenueID, Court: body.Court, StartAt: body.StartAt, EndAt: body.EndAt, MinLevel: body.MinLevel, MaxLevel: body.MaxLevel, Capacity: body.Capacity, FeeMinor: body.FeeMinor, DepositMinor: body.DepositMinor, Currency: body.Currency, PaymentRecipient: body.PaymentRecipient, PaymentInstructions: body.PaymentInstructions, PaymentInstructionVersion: body.PaymentInstructionVersion, JoinMode: body.JoinMode, HostPlays: body.HostPlays, CourtAttested: body.CourtAttested}
}

func (routes R1Routes) createMatch(w http.ResponseWriter, r *http.Request) {
	var body matchDraftRequest
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	match, err := routes.Matches.Create(r.Context(), principal.AccountID, body.draft())
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, matchResponse(match))
}

func (routes R1Routes) updateMatch(w http.ResponseWriter, r *http.Request) {
	var body matchDraftRequest
	if !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	match, err := routes.Matches.UpdateDraft(r.Context(), principal.AccountID, r.PathValue("matchID"), body.draft())
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, matchResponse(match))
}

func (routes R1Routes) publishMatch(w http.ResponseWriter, r *http.Request) {
	principal, _ := PrincipalFromContext(r.Context())
	match, err := routes.Matches.Publish(r.Context(), principal.AccountID, r.PathValue("matchID"))
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, matchResponse(match))
}

func (routes R1Routes) joinMatch(w http.ResponseWriter, r *http.Request) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "idempotency_key_required", "A valid Idempotency-Key header is required.")
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	participation, err := routes.Matches.Join(r.Context(), principal.AccountID, r.PathValue("matchID"), key)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, participationResponse(participation))
}

func (routes R1Routes) approveParticipation(w http.ResponseWriter, r *http.Request) {
	routes.decideParticipation(w, r, "approve")
}

func (routes R1Routes) rejectParticipation(w http.ResponseWriter, r *http.Request) {
	routes.decideParticipation(w, r, "reject")
}

func (routes R1Routes) removeParticipation(w http.ResponseWriter, r *http.Request) {
	routes.decideParticipation(w, r, "remove")
}

func (routes R1Routes) decideParticipation(w http.ResponseWriter, r *http.Request, action string) {
	key, ok := idempotencyKey(r)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "idempotency_key_required", "A valid Idempotency-Key header is required.")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if action != "approve" && !DecodeJSON(w, r, &body) {
		return
	}
	principal, _ := PrincipalFromContext(r.Context())
	var participation matches.Participation
	var err error
	switch action {
	case "approve":
		participation, err = routes.Matches.Approve(r.Context(), principal.AccountID, r.PathValue("matchID"), r.PathValue("participationID"), key)
	case "reject":
		participation, err = routes.Matches.Reject(r.Context(), principal.AccountID, r.PathValue("matchID"), r.PathValue("participationID"), key, body.Reason)
	case "remove":
		participation, err = routes.Matches.Remove(r.Context(), principal.AccountID, r.PathValue("matchID"), r.PathValue("participationID"), key, body.Reason)
	}
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, participationResponse(participation))
}

func (routes R1Routes) listMatches(w http.ResponseWriter, r *http.Request) {
	limit, ok := queryInt(r, "limit", 20)
	if !ok {
		WriteError(w, r, http.StatusBadRequest, "invalid_filter", "A query filter is invalid.")
		return
	}
	filters := matches.Filters{Area: r.URL.Query().Get("area"), Format: r.URL.Query().Get("format"), Limit: limit, Cursor: r.URL.Query().Get("cursor")}
	if !parseOptionalTime(r, "from", &filters.From) || !parseOptionalTime(r, "to", &filters.To) || !parseOptionalInt(r, "minLevel", &filters.MinLevel) || !parseOptionalInt(r, "maxLevel", &filters.MaxLevel) || !parseOptionalInt64(r, "maxCostMinor", &filters.MaxCostMinor) {
		WriteError(w, r, http.StatusBadRequest, "invalid_filter", "A query filter is invalid.")
		return
	}
	page, err := routes.Matches.List(r.Context(), filters)
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	items := make([]map[string]any, 0, len(page.Matches))
	for _, match := range page.Matches {
		items = append(items, matchResponse(match))
	}
	writeJSON(w, map[string]any{"items": items, "nextCursor": page.NextCursor})
}

func (routes R1Routes) matchDetail(w http.ResponseWriter, r *http.Request) {
	match, err := routes.Matches.Detail(r.Context(), r.PathValue("matchID"))
	if err != nil {
		routes.writeDomainError(w, r, err)
		return
	}
	writeJSON(w, matchResponse(match))
}

func matchResponse(match matches.Match) map[string]any {
	return map[string]any{"id": match.ID, "hostId": match.HostID, "title": match.Title, "description": match.Description, "format": match.Format, "style": match.Style, "rules": match.Rules, "venue": map[string]any{"id": match.Venue.ID, "name": match.Venue.Name, "address": match.Venue.Address, "area": match.Venue.Area, "timeZone": match.Venue.TimeZone, "court": match.Venue.Court}, "startAt": match.StartAt, "endAt": match.EndAt, "minLevel": match.MinLevel, "maxLevel": match.MaxLevel, "capacity": match.Capacity, "occupied": match.Occupied, "feeMinor": match.FeeMinor, "depositMinor": match.DepositMinor, "currency": match.Currency, "joinMode": match.JoinMode, "hostPlays": match.HostPlays, "cancellationPolicyVersion": match.CancellationPolicyVersion, "courtAttestation": map[string]any{"source": "HOST", "attested": match.CourtAttested, "attestedAt": match.CourtAttestedAt, "bookingGuaranteed": false}, "status": match.Status, "updatedAt": match.UpdatedAt}
}

func participationResponse(participation matches.Participation) map[string]any {
	response := map[string]any{"id": participation.ID, "matchId": participation.MatchID, "playerId": participation.PlayerID, "status": participation.Status, "decisionReason": participation.DecisionReason, "createdAt": participation.CreatedAt, "updatedAt": participation.UpdatedAt}
	if participation.Hold != nil {
		response["hold"] = map[string]any{"status": participation.Hold.Status, "expiresAt": participation.Hold.ExpiresAt, "transferReportedAt": participation.Hold.TransferReportedAt}
	}
	return response
}

func queryInt(r *http.Request, name string, fallback int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil && value > 0 && value <= 100
}

func parseOptionalTime(r *http.Request, name string, target **time.Time) bool {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return true
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return false
	}
	value = value.UTC()
	*target = &value
	return true
}

func parseOptionalInt(r *http.Request, name string, target **int) bool {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return false
	}
	*target = &value
	return true
}

func parseOptionalInt64(r *http.Request, name string, target **int64) bool {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return false
	}
	*target = &value
	return true
}

func (routes R1Routes) writeDomainError(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, "internal_error", "Đã xảy ra lỗi không mong đợi."
	switch {
	case errors.Is(err, identity.ErrInvalidCredentials):
		status, code, message = http.StatusUnauthorized, "invalid_credentials", "Email hoặc mật khẩu không hợp lệ."
	case errors.Is(err, identity.ErrInvalidToken):
		status, code, message = http.StatusBadRequest, "invalid_or_expired_token", "Liên kết hoặc token không hợp lệ hoặc đã hết hạn."
	case errors.Is(err, identity.ErrRateLimited):
		status, code, message = http.StatusTooManyRequests, "rate_limited", "Bạn đã thử quá nhiều lần. Vui lòng thử lại sau."
	case errors.Is(err, identity.ErrEmailExists):
		status, code, message = http.StatusConflict, "email_already_registered", "Email này đã được đăng ký."
	case errors.Is(err, identity.ErrAccountUnavailable):
		status, code, message = http.StatusForbidden, "account_unavailable", "Tài khoản hiện không thể thực hiện thao tác này."
	case errors.Is(err, identity.ErrForbidden), errors.Is(err, venues.ErrForbidden), errors.Is(err, matches.ErrForbidden), errors.Is(err, payments.ErrForbidden), errors.Is(err, notifications.ErrForbidden):
		status, code, message = http.StatusForbidden, "forbidden", "Bạn không có quyền thực hiện thao tác này."
	case errors.Is(err, identity.ErrConflict), errors.Is(err, players.ErrConflict), errors.Is(err, matches.ErrIdempotencyConflict), errors.Is(err, payments.ErrConflict), errors.Is(err, payments.ErrIdempotencyConflict), errors.Is(err, payments.ErrRefundLimit):
		status, code, message = http.StatusConflict, "conflict", "Dữ liệu đã thay đổi hoặc yêu cầu xung đột."
	case errors.Is(err, players.ErrUnderage):
		status, code, message = http.StatusUnprocessableEntity, "adult_eligibility_required", "Pilot chỉ dành cho người chơi từ 18 tuổi."
	case errors.Is(err, players.ErrDateOfBirthFixed):
		status, code, message = http.StatusConflict, "date_of_birth_locked", "Ngày sinh sau khi hoàn tất onboarding chỉ có thể sửa qua quy trình hỗ trợ."
	case errors.Is(err, players.ErrNotFound), errors.Is(err, venues.ErrNotFound), errors.Is(err, venues.ErrNotPublished), errors.Is(err, matches.ErrNotFound), errors.Is(err, payments.ErrNotFound), errors.Is(err, notifications.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "Không tìm thấy dữ liệu yêu cầu."
	case errors.Is(err, matches.ErrFull):
		status, code, message = http.StatusConflict, "match_full", "Kèo đã hết chỗ."
	case errors.Is(err, matches.ErrCapacityBelowOccupied):
		status, code, message = http.StatusConflict, "capacity_below_occupied", "Sức chứa không thể thấp hơn số suất đang được giữ hoặc đã xác nhận."
	case errors.Is(err, matches.ErrScheduleConflict):
		status, code, message = http.StatusConflict, "schedule_conflict", "Bạn đã có kèo trùng thời gian."
	case errors.Is(err, matches.ErrAlreadyParticipating):
		status, code, message = http.StatusConflict, "already_participating", "Bạn đã có lượt tham gia kèo này."
	case errors.Is(err, matches.ErrNotOpen):
		status, code, message = http.StatusConflict, "match_not_open", "Kèo hiện không còn nhận người chơi."
	case errors.Is(err, matches.ErrHoldExpired):
		status, code, message = http.StatusConflict, "payment_hold_expired", "Thời hạn giữ chỗ đã hết; khoản tiền được xử lý riêng nếu Host xác nhận đã nhận."
	case errors.Is(err, identity.ErrInvalidInput), errors.Is(err, players.ErrInvalidProfile), errors.Is(err, venues.ErrInvalid), errors.Is(err, matches.ErrInvalid), errors.Is(err, payments.ErrInvalid), errors.Is(err, notifications.ErrInvalid):
		status, code, message = http.StatusBadRequest, "validation_failed", "Dữ liệu gửi lên không hợp lệ."
	}
	WriteError(w, r, status, code, message)
}

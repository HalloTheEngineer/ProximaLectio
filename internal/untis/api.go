package untis

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type ResourceType string

const (
	ResourceStudent ResourceType = "STUDENT"
	ResourceClass   ResourceType = "CLASS"
	ResourceTeacher ResourceType = "TEACHER"
	ResourceRoom    ResourceType = "ROOM"
	ResourceSubject ResourceType = "SUBJECT"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	school     string
	username   string
	password   string
	token      string
	claims     *JWTClaims
}

type JWTClaims struct {
	TenantID string `json:"tenant_id"`
	Sub      string `json:"sub"`
	Roles    string `json:"roles"`
	Iss      string `json:"iss"`
	Locale   string `json:"locale"`
	UserID   int    `json:"user_id"`
	Host     string `json:"host"`
	Sn       string `json:"sn"`
	Exp      int64  `json:"exp"`
	Username string `json:"username"`
	PersonID int    `json:"person_id"`
}

type SchoolYear struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	DateRange struct {
		Start string `json:"start"` // Format: "2025-08-27"
		End   string `json:"end"`   // Format: "2026-07-10"
	} `json:"dateRange"`
}

type Absence struct {
	ID           int    `json:"id"`
	StartDate    int    `json:"startDate"` // Format: YYYYMMDD
	EndDate      int    `json:"endDate"`   // Format: YYYYMMDD
	StartTime    int    `json:"startTime"` // Format: HHMM
	EndTime      int    `json:"endTime"`   // Format: HHMM
	Reason       string `json:"reason"`
	ReasonID     int    `json:"reasonId"`
	Text         string `json:"text"`
	IsExcused    bool   `json:"isExcused"`
	ExcuseStatus string `json:"excuseStatus"`
	Excuse       *struct {
		ID           int    `json:"id"`
		Text         string `json:"text"`
		ExcuseDate   int    `json:"excuseDate"`
		ExcuseStatus string `json:"excuseStatus"`
		IsExcused    bool   `json:"isExcused"`
	} `json:"excuse"`
}

type AbsenceResponse struct {
	Data struct {
		Absences []Absence `json:"absences"`
	} `json:"data"`
}

type TimetableEntry struct {
	Days []struct {
		Date        string       `json:"date"`
		GridEntries []LessonSlot `json:"gridEntries"`
	} `json:"days"`
}

type LessonSlot struct {
	IDs      []int `json:"ids"`
	Duration struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"duration"`
	Status    string     `json:"status"`
	Position1 []Position `json:"position1"`
	Position2 []Position `json:"position2"`
	Position3 []Position `json:"position3"`
	Position4 []Position `json:"position4"`
	Position5 []Position `json:"position5"`
	Position6 []Position `json:"position6"`
	Position7 []Position `json:"position7"`
}

type Position struct {
	Current *struct {
		Type      string `json:"type"`
		ShortName string `json:"shortName"`
		LongName  string `json:"longName"`
	} `json:"current"`
}

type AppDataResponse struct {
	User struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Email  string `json:"email"`
		Person struct {
			ID          int    `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"person"`
	} `json:"user"`
}

type Exam struct {
	ID        int    `json:"id"`
	Date      int    `json:"date"`      // Format: YYYYMMDD
	StartTime int    `json:"startTime"` // Format: HHMM
	EndTime   int    `json:"endTime"`   // Format: HHMM
	Subject   string `json:"subject"`
	Name      string `json:"name"`
	ExamType  string `json:"examType"`
}

type Homework struct {
	ID        int    `json:"id"`
	LessonID  int    `json:"lessonId"`
	Date      int    `json:"date"`
	DueDate   int    `json:"dueDate"`
	Text      string `json:"text"`
	Completed bool   `json:"completed"`
}

type HomeworkLesson struct {
	ID      int    `json:"id"`
	Subject string `json:"subject"`
}

type HomeworkTeacher struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type HomeworkResponse struct {
	Data struct {
		Homeworks []Homework        `json:"homeworks"`
		Lessons   []HomeworkLesson  `json:"lessons"`
		Teachers  []HomeworkTeacher `json:"teachers"`
	} `json:"data"`
}

type loginResponse struct {
	State    string `json:"state"`
	SwitchUI bool   `json:"switchUI"`
}

// NewClient initializes the WebUntis client.
// It returns an error if the baseURL is empty.
func NewClient(school, username, password, baseURL string) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("WebUntis base URL cannot be empty")
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create cookie jar: %w", err)
	}

	return &Client{
		httpClient: &http.Client{Jar: jar, Timeout: 30 * time.Second},
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		school:     school,
		username:   username,
		password:   password,
	}, nil
}

func (c *Client) Authenticate(ctx context.Context) error {
	loginURL := fmt.Sprintf("%s/j_spring_security_check", c.baseURL)

	data := url.Values{}
	data.Set("school", c.school)
	data.Set("j_username", c.username)
	data.Set("j_password", c.password)
	data.Set("token", "")

	req, err := http.NewRequestWithContext(ctx, "POST", loginURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	loginClient := &http.Client{
		Jar:     c.httpClient.Jar,
		Timeout: c.httpClient.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := loginClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusSeeOther {
		return errors.New("authentication failed: invalid credentials (302 redirect)")
	}

	var loginStatus loginResponse
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response body: %w", err)
	}
	if err := json.Unmarshal(bodyBytes, &loginStatus); err == nil {
		if strings.ToUpper(loginStatus.State) == "FAILED" {
			return errors.New("authentication failed: invalid credentials")
		}
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed: unexpected status %d", resp.StatusCode)
	}

	tokenURL := fmt.Sprintf("%s/api/token/new", c.baseURL)
	reqT, err := http.NewRequestWithContext(ctx, "GET", tokenURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}
	respT, err := c.httpClient.Do(reqT)
	if err != nil {
		return err
	}
	defer respT.Body.Close()

	if respT.StatusCode != http.StatusOK {
		return errors.New("authentication failed: could not retrieve bearer token")
	}

	tokenBody, err := io.ReadAll(respT.Body)
	if err != nil {
		return fmt.Errorf("failed to read token response body: %w", err)
	}
	c.token = strings.Trim(string(tokenBody), "\"")

	claims, err := parseJWTClaims(c.token)
	if err != nil {
		return fmt.Errorf("failed to parse token claims: %w", err)
	}
	c.claims = claims

	return nil
}

func (c *Client) EnsureToken(ctx context.Context) error {
	if c.token == "" || c.claims == nil || time.Now().Unix() > c.claims.Exp-60 {
		return c.Authenticate(ctx)
	}
	return nil
}

func (c *Client) GetAppData(ctx context.Context) (*AppDataResponse, error) {
	var data AppDataResponse
	err := c.doREST(ctx, "GET", "/api/rest/view/v1/app/data", nil, &data)
	return &data, err
}

func (c *Client) GetSchoolYears(ctx context.Context) ([]SchoolYear, error) {
	var years []SchoolYear
	err := c.doREST(ctx, "GET", "/api/rest/view/v1/schoolyears", nil, &years)
	return years, err
}

func (c *Client) GetAbsences(ctx context.Context, start, end time.Time) ([]Absence, error) {
	var response AbsenceResponse

	sStr := start.Format("20060102")
	eStr := end.Format("20060102")

	path := fmt.Sprintf("/api/classreg/absences/students?startDate=%s&endDate=%s&studentId=%d&excuseStatusId=-1",
		sStr, eStr, c.claims.PersonID)

	err := c.doREST(ctx, "GET", path, nil, &response)
	return response.Data.Absences, err
}

func (c *Client) GetMyTimetable(ctx context.Context, start, end time.Time) (*TimetableEntry, error) {
	sStr := start.Format("2006-01-02")
	eStr := end.Format("2006-01-02")

	path := fmt.Sprintf("/api/rest/view/v1/timetable/entries?start=%s&end=%s&format=7&resourceType=STUDENT&resources=%d&timetableType=MY_TIMETABLE&layout=START_TIME",
		sStr, eStr, c.claims.PersonID)

	var data TimetableEntry
	err := c.doREST(ctx, "GET", path, nil, &data)
	return &data, err
}

func (c *Client) GetHomeworks(ctx context.Context, start, end time.Time) (*HomeworkResponse, error) {
	sStr := start.Format("20060102")
	eStr := end.Format("20060102")

	path := fmt.Sprintf("/api/homeworks/lessons?startDate=%s&endDate=%s", sStr, eStr)

	var resp HomeworkResponse
	err := c.doREST(ctx, "GET", path, nil, &resp)
	return &resp, err
}

func (c *Client) doREST(ctx context.Context, method, path string, body io.Reader, v interface{}) error {
	if err := c.EnsureToken(ctx); err != nil {
		return err
	}

	fullURL := c.baseURL + path
	req, _ := http.NewRequestWithContext(ctx, method, fullURL, body)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Tenant-Id", c.claims.TenantID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		if err := c.Authenticate(ctx); err != nil {
			return err
		}
		return c.doREST(ctx, method, path, body, v)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("untis api error: status %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

func parseJWTClaims(token string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid token")
	}
	payload := parts[1]
	if l := len(payload) % 4; l > 0 {
		payload += strings.Repeat("=", 4-l)
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	var claims JWTClaims
	err = json.Unmarshal(decoded, &claims)
	return &claims, err
}

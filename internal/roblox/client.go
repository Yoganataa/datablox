package roblox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// PlaceURLPattern matches https://www.roblox.com/games/<id>/<name> style URLs.
var PlaceURLPattern = regexp.MustCompile(`roblox\.com/games/(\d+)`)

const baseURL = "https://apis.roblox.com"
const gamesURL = "https://games.roblox.com"
const thumbnailsURL = "https://thumbnails.roblox.com"

type Client struct {
	http *http.Client
}

func New(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &Client{
		http: &http.Client{Timeout: timeout},
	}
}

// ExtractPlaceID pulls the PlaceID out of a Roblox game URL or bare ID.
func ExtractPlaceID(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if m := PlaceURLPattern.FindStringSubmatch(raw); len(m) == 2 {
		return strconv.ParseInt(m[1], 10, 64)
	}
	if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
		return id, nil
	}
	return 0, fmt.Errorf("bukan URL atau ID Roblox yang valid: %q", raw)
}

type universeResponse struct {
	UniverseID int64 `json:"universeId"`
}

// ResolveUniverseID maps a PlaceID to its UniverseID.
func (c *Client) ResolveUniverseID(ctx context.Context, placeID int64) (int64, error) {
	endpoint := fmt.Sprintf("%s/universes/v1/places/%d/universe", baseURL, placeID)
	var out universeResponse
	if err := c.get(ctx, endpoint, nil, &out); err != nil {
		return 0, err
	}
	if out.UniverseID == 0 {
		return 0, fmt.Errorf("experience not found for place %d", placeID)
	}
	return out.UniverseID, nil
}

type gamesResponse struct {
	Data []GameDetail `json:"data"`
}

type GameDetail struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Creator     struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"creator"`
	Playing    int   `json:"playing"`
	Visits     int64 `json:"visits"`
	MaxPlayers int   `json:"maxPlayers"`
	Created    string `json:"created"`
	Updated    string `json:"updated"`
}

type votesResponse struct {
	Data []struct {
		ID        int64 `json:"id"`
		UpVotes   int64 `json:"upVotes"`
		DownVotes int64 `json:"downVotes"`
	} `json:"data"`
}

// GetGameDetail fetches metadata for a list of universe IDs (batch, max 100).
func (c *Client) GetGameDetail(ctx context.Context, universeIDs ...int64) ([]GameDetail, error) {
	strs := make([]string, 0, len(universeIDs))
	for _, id := range universeIDs {
		strs = append(strs, strconv.FormatInt(id, 10))
	}
	q := url.Values{}
	q.Set("universeIds", strings.Join(strs, ","))
	var out gamesResponse
	if err := c.get(ctx, gamesURL+"/v1/games", q, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// GetVotes fetches upVotes/downVotes for a batch of universe IDs.
func (c *Client) GetVotes(ctx context.Context, universeIDs ...int64) (map[int64]struct{ Up, Down int64 }, error) {
	strs := make([]string, 0, len(universeIDs))
	for _, id := range universeIDs {
		strs = append(strs, strconv.FormatInt(id, 10))
	}
	q := url.Values{}
	q.Set("universeIds", strings.Join(strs, ","))
	var out votesResponse
	if err := c.get(ctx, gamesURL+"/v1/games/votes", q, &out); err != nil {
		return nil, err
	}
	m := make(map[int64]struct{ Up, Down int64 }, len(out.Data))
	for _, d := range out.Data {
		m[d.ID] = struct{ Up, Down int64 }{d.UpVotes, d.DownVotes}
	}
	return m, nil
}

type thumbnailsResponse struct {
	Data []struct {
		ImageURL string `json:"imageUrl"`
		State    string `json:"state"`
	} `json:"data"`
}

// GetThumbnailURL returns the icon URL for the given universe.
func (c *Client) GetThumbnailURL(ctx context.Context, universeID int64) (string, error) {
	for _, size := range []string{"512x512", "150x150"} {
		q := url.Values{}
		q.Set("universeIds", strconv.FormatInt(universeID, 10))
		q.Set("size", size)
		q.Set("format", "Png")
		q.Set("isCircular", "false")
		var out thumbnailsResponse
		if err := c.get(ctx, thumbnailsURL+"/v1/games/icons", q, &out); err != nil {
			continue
		}
		if len(out.Data) > 0 && out.Data[0].ImageURL != "" && out.Data[0].State != "Error" {
			return out.Data[0].ImageURL, nil
		}
	}
	return "", nil
}

func (c *Client) get(ctx context.Context, endpoint string, params url.Values, out any) error {
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("resource not found: %s", endpoint)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("Roblox rate-limit: please try again shortly")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("Roblox API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

type UserGroup struct {
	GroupID   int64  `json:"groupId"`
	GroupName string `json:"groupName"`
	Rank      int    `json:"rank"`
	RoleID    int64  `json:"roleId"`
	RoleName  string `json:"roleName"`
}

func (c *Client) GetUserGroups(ctx context.Context, robloxID int64) ([]UserGroup, error) {
	endpoint := fmt.Sprintf("https://groups.roblox.com/v2/users/%d/groups/roles", robloxID)
	var out struct {
		Data []struct {
			Group struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"group"`
			Role struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
				Rank int    `json:"rank"`
			} `json:"role"`
		} `json:"data"`
	}
	if err := c.get(ctx, endpoint, nil, &out); err != nil {
		return nil, err
	}
	groups := make([]UserGroup, 0, len(out.Data))
	for _, d := range out.Data {
		groups = append(groups, UserGroup{GroupID: d.Group.ID, GroupName: d.Group.Name, Rank: d.Role.Rank, RoleID: d.Role.ID, RoleName: d.Role.Name})
	}
	return groups, nil
}


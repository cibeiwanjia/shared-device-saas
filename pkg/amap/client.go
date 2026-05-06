package amap

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	defaultBaseURL = "https://restapi.amap.com"
	defaultTimeout = 5 * time.Second
)

// Client 高德地图 API 客户端
type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
	log     *log.Helper
}

// NewClient 创建高德地图客户端
func NewClient(apiKey string, logger log.Logger) *Client {
	if logger == nil {
		logger = log.DefaultLogger
	}
	return &Client{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		client:  &http.Client{Timeout: defaultTimeout},
		log:     log.NewHelper(logger),
	}
}

// ========================================
// 1. 周边搜索 (Around Search)
// ========================================

// AroundSearchRequest 周边搜索参数
type AroundSearchRequest struct {
	Location  string  // 中心点坐标 "lng,lat"
	Keywords  string  // 搜索关键词，如"快递柜|储物柜"
	Radius    int     // 搜索半径（米），默认 3000，最大 50000
	Page      int     // 页码
	PageSize  int     // 每页数量，默认 20，最大 25
	Types     string  // POI 类型，可选
	SortRule  string  // 排序规则: distance(距离)/weight(综合)
}

// POI 兴趣点
type POI struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Address  string `json:"address"`
	Location string `json:"location"` // "lng,lat"
	Distance int    `json:"distance"` // 距离（米）
	Pname    string `json:"pname"`    // 省
	Cityname string `json:"cityname"` // 市
	Adname   string `json:"adname"`   // 区
}

// AroundSearchResponse 周边搜索响应
type AroundSearchResponse struct {
	Status string `json:"status"`
	Info   string `json:"info"`
	Count  int    `json:"count"`
	POIS   []POI  `json:"pois"`
}

// AroundSearch 周边搜索
func (c *Client) AroundSearch(req *AroundSearchRequest) (*AroundSearchResponse, error) {
	if req.Radius == 0 {
		req.Radius = 3000
	}
	if req.PageSize == 0 {
		req.PageSize = 20
	}
	if req.SortRule == "" {
		req.SortRule = "distance"
	}

	params := url.Values{}
	params.Set("key", c.apiKey)
	params.Set("location", req.Location)
	params.Set("radius", fmt.Sprintf("%d", req.Radius))
	params.Set("sortrule", req.SortRule)
	params.Set("offset", fmt.Sprintf("%d", req.PageSize))
	params.Set("page", fmt.Sprintf("%d", req.Page))

	if req.Keywords != "" {
		params.Set("keywords", req.Keywords)
	}
	if req.Types != "" {
		params.Set("types", req.Types)
	}
	params.Set("extensions", "all")

	var result AroundSearchResponse
	if err := c.doGet("/v3/place/around", params, &result); err != nil {
		return nil, err
	}

	if result.Status != "1" {
		return nil, fmt.Errorf("amap around search: %s", result.Info)
	}

	return &result, nil
}

// ========================================
// 2. 地理编码 (Geocoding)
// ========================================

// GeoCode 地理编码结果
type GeoCode struct {
	FormattedAddress string `json:"formatted_address"`
	Country          string `json:"country"`
	Province         string `json:"province"`
	City             string `json:"city"`
	District         string `json:"district"`
	Location         string `json:"location"` // "lng,lat"
	Level            string `json:"level"`
}

// GeoCodeResponse 地理编码响应
type GeoCodeResponse struct {
	Status   string    `json:"status"`
	Info     string    `json:"info"`
	Count    int       `json:"count"`
	GeoCodes []GeoCode `json:"geocodes"`
}

// GeoCode 地理编码（地址 → 经纬度）
func (c *Client) GeoCode(address, city string) (*GeoCodeResponse, error) {
	params := url.Values{}
	params.Set("key", c.apiKey)
	params.Set("address", address)
	if city != "" {
		params.Set("city", city)
	}

	var result GeoCodeResponse
	if err := c.doGet("/v3/geocode/geo", params, &result); err != nil {
		return nil, err
	}
	if result.Status != "1" {
		return nil, fmt.Errorf("amap geocode: %s", result.Info)
	}
	return &result, nil
}

// ========================================
// 3. 逆地理编码 (Reverse Geocoding)
// ========================================

// ReGeoCodeResult 逆地理编码结果
type ReGeoCodeResult struct {
	FormattedAddress string `json:"formatted_address"`
	Country          string `json:"country"`
	Province         string `json:"province"`
	City             string `json:"city"`
	District         string `json:"district"`
	Adcode           string `json:"adcode"`
}

// ReGeoCodeResponse 逆地理编码响应
type ReGeoCodeResponse struct {
	Status     string          `json:"status"`
	Info       string          `json:"info"`
	ReGeoCode  ReGeoCodeResult `json:"regeocode"`
}

// ReGeoCode 逆地理编码（经纬度 → 地址）
func (c *Client) ReGeoCode(location string) (*ReGeoCodeResponse, error) {
	params := url.Values{}
	params.Set("key", c.apiKey)
	params.Set("location", location)
	params.Set("extensions", "base")

	var result ReGeoCodeResponse
	if err := c.doGet("/v3/geocode/regeo", params, &result); err != nil {
		return nil, err
	}
	if result.Status != "1" {
		return nil, fmt.Errorf("amap regeocode: %s", result.Info)
	}
	return &result, nil
}

// ========================================
// 4. 骑行路径规划 (Cycling Route)
// ========================================

// CyclingRouteResponse 骑行路径响应
type CyclingRouteResponse struct {
	Status string `json:"status"`
	Info   string `json:"info"`
	Data   struct {
		Paths []struct {
			Distance int `json:"distance"` // 距离（米）
			Duration int `json:"duration"` // 时间（秒）
		} `json:"paths"`
	} `json:"data"`
}

// CyclingRoute 骑行路径规划
func (c *Client) CyclingRoute(origin, destination string) (*CyclingRouteResponse, error) {
	params := url.Values{}
	params.Set("key", c.apiKey)
	params.Set("origin", origin)     // "lng,lat"
	params.Set("destination", destination) // "lng,lat"

	var result CyclingRouteResponse
	if err := c.doGet("/v4/direction/bicycling", params, &result); err != nil {
		return nil, err
	}
	if result.Status != "1" {
		return nil, fmt.Errorf("amap cycling route: %s", result.Info)
	}
	return &result, nil
}

// ========================================
// HTTP 请求
// ========================================

func (c *Client) doGet(path string, params url.Values, result interface{}) error {
	fullURL := c.baseURL + path + "?" + params.Encode()

	c.log.Debugf("Amap request: %s", fullURL)

	resp, err := c.client.Get(fullURL)
	if err != nil {
		return fmt.Errorf("amap http get: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("amap read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Warnf("Amap non-200 response: status=%d body=%s", resp.StatusCode, string(body))
		return fmt.Errorf("amap http status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.Unmarshal(body, result); err != nil {
		c.log.Warnf("Amap response decode failed: %s", string(body))
		return fmt.Errorf("amap decode: %w", err)
	}

	return nil
}

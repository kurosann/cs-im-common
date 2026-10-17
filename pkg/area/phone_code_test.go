package area

import (
	"testing"
)

func TestGetCountryByPhone(t *testing.T) {
	tests := []struct {
		name        string
		phoneNumber string
		wantCount   int
		wantName    string
	}{
		{
			name:        "中国手机号码",
			phoneNumber: "+86 138 0013 8000",
			wantCount:   1,
			wantName:    "中国",
		},
		{
			name:        "美国手机号码",
			phoneNumber: "+1 555 123 4567",
			wantCount:   23,
			wantName:    "美国",
		},
		{
			name:        "俄罗斯手机号码",
			phoneNumber: "+7 900 123 4567",
			wantCount:   2,
			wantName:    "俄罗斯",
		},
		{
			name:        "英国手机号码",
			phoneNumber: "+44 7911 123456",
			wantCount:   1,
			wantName:    "英国",
		},
		{
			name:        "孟加拉国手机号码",
			phoneNumber: "+8801343262280",
			wantCount:   1,
			wantName:    "孟加拉国",
		},
		{
			name:        "无效号码",
			phoneNumber: "abc123",
			wantCount:   23, // 匹配到区号1的国家
			wantName:    "美国",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCountryByPhone(tt.phoneNumber)
			if len(got) != tt.wantCount {
				t.Errorf("GetCountryByPhone() 返回 %d 个国家，期望 %d 个", len(got), tt.wantCount)
				return
			}

			if tt.wantCount > 0 && got[0].CountryName != tt.wantName {
				t.Errorf("GetCountryByPhone() 国家名称 = %v, 期望 %v", got[0].CountryName, tt.wantName)
			}
		})
	}
}

func TestGetCountryByCode(t *testing.T) {
	tests := []struct {
		name      string
		code      int
		wantCount int
		wantName  string
	}{
		{
			name:      "中国区号",
			code:      86,
			wantCount: 1,
			wantName:  "中国",
		},
		{
			name:      "美国区号",
			code:      1,
			wantCount: 23,
			wantName:  "美国",
		},
		{
			name:      "不存在的区号",
			code:      999,
			wantCount: 0,
			wantName:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetCountryByCode(tt.code)
			if len(got) != tt.wantCount {
				t.Errorf("GetCountryByCode() 返回 %d 个国家，期望 %d 个", len(got), tt.wantCount)
				return
			}

			if tt.wantCount > 0 && got[0].CountryName != tt.wantName {
				t.Errorf("GetCountryByCode() 国家名称 = %v, 期望 %v", got[0].CountryName, tt.wantName)
			}
		})
	}
}

func TestSearchCountryByName(t *testing.T) {
	tests := []struct {
		name    string
		search  string
		wantMin int
	}{
		{
			name:    "搜索中国",
			search:  "中国",
			wantMin: 1,
		},
		{
			name:    "搜索美国",
			search:  "美国",
			wantMin: 1,
		},
		{
			name:    "模糊搜索",
			search:  "国",
			wantMin: 5,
		},
		{
			name:    "不存在的国家",
			search:  "不存在",
			wantMin: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SearchCountryByName(tt.search)
			if len(got) < tt.wantMin {
				t.Errorf("SearchCountryByName() 返回 %d 个国家，期望至少 %d 个", len(got), tt.wantMin)
			}
		})
	}
}

func TestIsValidPhoneFormat(t *testing.T) {
	tests := []struct {
		name  string
		phone string
		want  bool
	}{
		{
			name:  "有效的中国手机号码",
			phone: "13800138000",
			want:  true,
		},
		{
			name:  "有效的国际手机号码",
			phone: "+15551234567",
			want:  true,
		},
		{
			name:  "太短的号码",
			phone: "8801343262280",
			want:  false,
		},
		{
			name:  "太长的号码",
			phone: "12345678901234567890",
			want:  false,
		},
		{
			name:  "包含特殊字符的有效号码",
			phone: "+86-138-0013-8000",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidPhoneFormat(tt.phone); got != tt.want {
				t.Errorf("IsValidPhoneFormat() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

func TestGetAllCountries(t *testing.T) {
	countries := GetAllCountries()

	if len(countries) == 0 {
		t.Error("GetAllCountries() 返回空列表")
	}

	// 验证数据完整性
	chinaFound := false
	usFound := false

	for _, country := range countries {
		if country.CountryName == "中国" && country.PhoneCode == 86 {
			chinaFound = true
		}
		if country.CountryName == "美国" && country.PhoneCode == 1 {
			usFound = true
		}
	}

	if !chinaFound {
		t.Error("GetAllCountries() 未找到中国")
	}

	if !usFound {
		t.Error("GetAllCountries() 未找到美国")
	}
}

func BenchmarkGetCountryByPhone(b *testing.B) {
	phoneNumbers := []string{
		"+86 138 0013 8000",
		"+1 555 123 4567",
		"+44 7911 123456",
		"+33 6 12 34 56 78",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, phone := range phoneNumbers {
			GetCountryByPhone(phone)
		}
	}
}

package models

// helpers "davet.link/models/helpers" // JSONBMap veya StringArray gerekirse

// CardDetail dijital kartvizitin detaylarını içerir.
type CardDetail struct {
	BaseModel
	CardID uint `gorm:"uniqueIndex;not null"` // cards.id FK

	// --- Kartvizit Alanları (Örnekler) ---
	// Kişisel Bilgiler
	Prefix     string `gorm:"type:varchar(20)"` // Örn: Dr., Av., Mr., Ms.
	FirstName  string `gorm:"type:varchar(100);not null"`
	LastName   string `gorm:"type:varchar(100);not null"`
	Suffix     string `gorm:"type:varchar(20)"`  // Örn: Jr., Sr., PhD.
	Title      string `gorm:"type:varchar(100)"` // Ünvan (örn: Yazılım Geliştirici)
	Company    string `gorm:"type:varchar(150)"` // Şirket Adı
	Department string `gorm:"type:varchar(100)"` // Departman
	Bio        string `gorm:"type:text"`         // Kısa Biyografi

	// İletişim Bilgileri
	Email       string `gorm:"type:varchar(100);index"`
	PhoneNumber string `gorm:"type:varchar(30)"`
	Website     string `gorm:"type:varchar(255)"`
	Address     string `gorm:"type:text"`

	// Sosyal Medya Linkleri (JSONB veya ayrı tablo daha iyi olabilir)
	// Basitlik için şimdilik text alanları kullanalım:
	LinkedInURL  string `gorm:"type:varchar(255)"`
	TwitterURL   string `gorm:"type:varchar(255)"`
	GitHubURL    string `gorm:"type:varchar(255)"`
	InstagramURL string `gorm:"type:varchar(255)"`
	// Alternatif: SocialLinks helpers.JSONBMap `gorm:"type:jsonb"`

	// Görsel Öğeler
	ProfilePictureURL string `gorm:"type:varchar(500)"` // Profil fotoğrafı URL'si
	LogoURL           string `gorm:"type:varchar(500)"` // Şirket/Kişisel logo URL'si
	Theme             string `gorm:"type:varchar(50)"`  // Görsel tema adı/kodu
	PrimaryColor      string `gorm:"type:varchar(7)"`   // Tema rengi (örn: #FFFFFF)
	SecondaryColor    string `gorm:"type:varchar(7)"`   // Tema rengi

	// Ek Ayarlar
	AllowSaveContact bool `gorm:"default:true"` // vCard indirme izni
	// CustomFields helpers.JSONBMap `gorm:"type:jsonb"` // İsteğe bağlı özel alanlar
}

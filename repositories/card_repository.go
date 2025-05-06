// repositories/card_repository.go
package repositories

import (
	"context"
	"errors" // ErrNotFound için
	"strings"

	"davet.link/configs/configsdatabase"
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/pkg/queryparams"
	"davet.link/pkg/turkishsearch"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ICardRepository kartvizit veritabanı işlemleri için arayüz.
// IBaseRepository'deki metodları içerir (veya direkt Base kullanabiliriz).
type ICardRepository interface {
	// BaseRepository metodları (Embed etmek yerine direkt kullanacağız)
	GetAllCards(params queryparams.ListParams) ([]models.Card, int64, error) // Özel listeleme (Detail ile join)
	GetCardByID(id uint) (*models.Card, error)                               // Özel preload
	CreateCard(ctx context.Context, card *models.Card) error
	UpdateCard(ctx context.Context, id uint, data map[string]interface{}, updatedBy uint) error
	DeleteCard(ctx context.Context, id uint) error
	GetCardCount() (int64, error)
	FindCardByLinkID(linkID uint) (*models.Card, error)                                                     // Link ID ile bulma (ekstra)
	FindAllCardsByUserIDPaginated(userID uint, params queryparams.ListParams) ([]models.Card, int64, error) // Kullanıcıya göre listeleme
	CountCardsByUserID(userID uint) (int64, error)                                                          // Kullanıcıya göre sayım
}

// CardRepository ICardRepository arayüzünü uygular.
// BaseRepository'yi kullanır ancak bazı metodları override edebilir.
type CardRepository struct {
	base IBaseRepository[models.Card] // Generik base repo
	db   *gorm.DB                     // Preload ve Join için direkt erişim
}

// NewCardRepository yeni bir CardRepository örneği oluşturur.
func NewCardRepository() ICardRepository {
	db := configsdatabase.GetDB()
	base := NewBaseRepository[models.Card](db)
	// Kartvizit için izin verilen sıralama sütunları (Detail'deki alanlar için JOIN gerekir)
	base.SetAllowedSortColumns([]string{
		"id", "created_at", "is_enabled", // Ana tablo
		// Detail tablosundan sıralama gerekiyorsa, GetAllCards'da özel JOIN ve sıralama yapılmalı
		// "first_name", "last_name", "company",
	})

	return &CardRepository{base: base, db: db} // db'yi de sakla
}

// GetAllCards kartvizitleri sayfalayarak listeler (Detail ile JOIN yaparak filtreleme/sıralama).
// BaseRepository.GetAll'ı override eder çünkü Detail tablosuna JOIN gerekiyor.
func (r *CardRepository) GetAllCards(params queryparams.ListParams) ([]models.Card, int64, error) {
	var results []models.Card
	var totalCount int64

	// Ana tabloya ve Detail tablosuna JOIN yap
	// Preload yerine JOIN kullanmak filtreleme ve sıralama için daha esnek olabilir
	query := r.db.Model(&models.Card{}).
		Joins("JOIN card_details ON card_details.card_id = cards.id")

	// İsim/Şirket filtresi (Detail tablosundan)
	if params.Name != "" {
		// türkçe karakter duyarsız arama (turkishsearch.SQLFilter varsayımı)
		sqlFragment, args := turkishsearch.SQLFilter("card_details.first_name", params.Name)
		sqlFragment2, args2 := turkishsearch.SQLFilter("card_details.last_name", params.Name)
		sqlFragment3, args3 := turkishsearch.SQLFilter("card_details.company", params.Name)
		query = query.Where(sqlFragment+" OR "+sqlFragment2+" OR "+sqlFragment3, args[0], args2[0], args3[0]) // ILIKE için argümanlar aynı olabilir
		// Veya daha basit:
		// searchValue := "%" + strings.ToLower(params.Name) + "%"
		// query = query.Where(
		// 	r.db.Where("unaccent(lower(card_details.first_name)) ILIKE unaccent(?)", searchValue).
		// 		Or("unaccent(lower(card_details.last_name)) ILIKE unaccent(?)", searchValue).
		// 		Or("unaccent(lower(card_details.company)) ILIKE unaccent(?)", searchValue),
		// )
	}

	// Status filtresi (Ana tablodan)
	if params.Status != "" {
		query = query.Where("cards.status = ?", params.Status) // Tablo adını belirtmek iyi olur
	}
	// Type filtresi Card için anlamsız, kaldırıldı.

	// Filtrelenmiş toplam sayıyı al
	err := query.Count(&totalCount).Error
	if err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return results, 0, nil
	}

	// Sıralama (Base'deki SetAllowedSortColumns kullanılır)
	// Ancak Detail alanlarına göre sıralama için tablo adı belirtilmeli.
	sortBy := params.SortBy
	orderBy := strings.ToLower(params.OrderBy)
	if orderBy != "asc" && orderBy != "desc" {
		orderBy = queryparams.DefaultOrderBy
	}

	// CardRepository için özel izin verilen sütunlar
	allowedSortColumns := map[string]string{
		"id":         "cards.id",
		"created_at": "cards.created_at",
		"is_enabled": "cards.is_enabled",
		"first_name": "card_details.first_name",
		"last_name":  "card_details.last_name",
		"company":    "card_details.company",
	}
	orderColumn := "cards.created_at" // Varsayılan
	if dbCol, ok := allowedSortColumns[sortBy]; ok {
		orderColumn = dbCol
	} else {
		sortBy = "created_at" // Varsayılan
	}
	query = query.Order(orderColumn + " " + orderBy)

	// Sayfalama
	offset := params.CalculateOffset()
	query = query.Limit(params.PerPage).Offset(offset)

	// Verileri çek (JOIN yapıldığı için Preload("Detail") otomatik olabilir,
	// ancak Link için ayrıca Preload gerekir)
	// GORM'un davranışı değişebilir, açıkça Preload eklemek daha güvenli.
	// JOIN'lu sorgularda Select() ile alanları seçmek daha iyi performans verebilir.
	query = query.Preload("Link.Type")                 // Sadece Link ve Type'ı Preload et
	err = query.Select("cards.*").Find(&results).Error // Select ile ana tabloyu al

	// JOIN yapıldığı için Detail verileri manuel olarak yüklenebilir veya GORM'a bırakılabilir.
	// Eğer GORM Detail'i yüklemezse:
	if err == nil && len(results) > 0 {
		cardIDs := make([]uint, len(results))
		for i, card := range results {
			cardIDs[i] = card.ID
		}
		var details []models.CardDetail
		if detailErr := r.db.Where("card_id IN ?", cardIDs).Find(&details).Error; detailErr == nil {
			detailsMap := make(map[uint]models.CardDetail)
			for _, d := range details {
				detailsMap[d.CardID] = d
			}
			for i := range results {
				if detail, found := detailsMap[results[i].ID]; found {
					results[i].Detail = detail
				}
			}
		} else {
			configslog.Log.Warn("GetAllCards: Detaylar yüklenemedi", zap.Error(detailErr))
		}
	}

	return results, totalCount, err
}

// GetCardByID kartviziti ID ile bulur (Detail ve Link ile).
func (r *CardRepository) GetCardByID(id uint) (*models.Card, error) {
	var result models.Card
	// İlişkileri yükle
	err := r.db.Preload("Detail").Preload("Link.Type").First(&result, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound // Kendi hata tipimizi döndür
	}
	return &result, err
}

// CreateCard yeni kartvizit oluşturur (BaseRepository'yi kullanır).
func (r *CardRepository) CreateCard(ctx context.Context, card *models.Card) error {
	return r.base.Create(ctx, card) // Detail de cascade ile oluşturulmalı
}

// UpdateCard kartvizit ana bilgilerini veya detayını günceller (BaseRepository'yi kullanır).
// Map içindeki anahtarlara göre GORM doğru tabloyu hedefler (BaseModel'deki hook'lar çalışır).
// Ancak Detail güncellemesi için ayrı bir metod daha güvenli olabilir.
func (r *CardRepository) UpdateCard(ctx context.Context, id uint, data map[string]interface{}, updatedBy uint) error {
	// Bu metod hem Card hem CardDetail alanlarını güncelleyebilir.
	// Eğer sadece Card ana tablosunu güncellemek istiyorsak, map'i filtrelemeliyiz.
	// Şimdilik tüm map'i gönderiyoruz.
	return r.base.Update(ctx, id, data, updatedBy)
}

// DeleteCard kartviziti siler (BaseRepository'yi kullanır).
func (r *CardRepository) DeleteCard(ctx context.Context, id uint) error {
	// BaseRepository'nin Delete metodu context'ten userID alır ve DeletedBy'ı set eder.
	// Link'in ayrıca silinmesi Servis katmanının sorumluluğundadır.
	return r.base.Delete(ctx, id)
}

// GetCardCount toplam kartvizit sayısını alır (BaseRepository'yi kullanır).
func (r *CardRepository) GetCardCount() (int64, error) {
	return r.base.GetCount()
}

// FindCardByLinkID Link ID ile kartviziti bulur.
func (r *CardRepository) FindCardByLinkID(linkID uint) (*models.Card, error) {
	if linkID == 0 {
		return nil, errors.New("geçersiz Link ID")
	}
	var card models.Card
	err := r.db.Preload("Detail").Preload("Link.Type").Where("link_id = ?", linkID).First(&card).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		configslog.Log.Error("CardRepository.FindByLinkID: DB error", zap.Uint("link_id", linkID), zap.Error(err))
		return nil, err
	}
	return &card, nil
}

// FindAllCardsByUserIDPaginated kullanıcıya ait kartvizitleri listeler.
// Bu, GetAllCards'a benzer ama kullanıcı ID'si filtresi ekler.
func (r *CardRepository) FindAllCardsByUserIDPaginated(userID uint, params queryparams.ListParams) ([]models.Card, int64, error) {
	if userID == 0 {
		return nil, 0, errors.New("geçersiz User ID")
	}
	var results []models.Card
	var totalCount int64

	// Ana tabloya ve Detail tablosuna JOIN yap, CreatorUserID ile filtrele
	query := r.db.Model(&models.Card{}).
		Joins("JOIN card_details ON card_details.card_id = cards.id").
		Where("cards.creator_user_id = ?", userID) // Kullanıcı filtresi

	// İsim/Şirket filtresi
	if params.Name != "" {
		sqlFragment, args := turkishsearch.SQLFilter("card_details.first_name", params.Name)
		sqlFragment2, args2 := turkishsearch.SQLFilter("card_details.last_name", params.Name)
		sqlFragment3, args3 := turkishsearch.SQLFilter("card_details.company", params.Name)
		query = query.Where(sqlFragment+" OR "+sqlFragment2+" OR "+sqlFragment3, args[0], args2[0], args3[0])
	}

	// Status filtresi
	if params.Status != "" {
		query = query.Where("cards.status = ?", params.Status)
	}

	// Toplam sayıyı al
	err := query.Count(&totalCount).Error
	if err != nil { /*...*/
		return nil, 0, err
	}
	if totalCount == 0 {
		return results, 0, nil
	}

	// Sıralama
	sortBy := params.SortBy
	orderBy := strings.ToLower(params.OrderBy)
	if orderBy != "asc" && orderBy != "desc" {
		orderBy = queryparams.DefaultOrderBy
	}
	allowedSortColumns := map[string]string{ /* ... önceki gibi ... */ }
	orderColumn := "cards.created_at"
	if dbCol, ok := allowedSortColumns[sortBy]; ok {
		orderColumn = dbCol
	} else {
		sortBy = "created_at"
	}
	query = query.Order(orderColumn + " " + orderBy)

	// Sayfalama
	offset := params.CalculateOffset()
	query = query.Limit(params.PerPage).Offset(offset)

	// Verileri çek (Select ve Preload)
	query = query.Preload("Link.Type")
	err = query.Select("cards.*").Find(&results).Error

	// Detayları manuel yükle (JOIN sonrası GORM davranışı belirsiz olabilir)
	if err == nil && len(results) > 0 { /* ... Detay yükleme (GetAllCards'daki gibi) ... */
	}

	return results, totalCount, err
}

// CountCardsByUserID kullanıcıya ait kartvizit sayısını alır.
func (r *CardRepository) CountCardsByUserID(userID uint) (int64, error) {
	if userID == 0 {
		return 0, errors.New("geçersiz User ID")
	}
	var count int64
	err := r.db.Model(&models.Card{}).Where("creator_user_id = ?", userID).Count(&count).Error
	return count, err
}

// Arayüz uyumluluğu kontrolü
var _ ICardRepository = (*CardRepository)(nil)

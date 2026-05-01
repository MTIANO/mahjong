package mahjong

type YakuType int

const (
	YakuRiichi  YakuType = iota // 立直
	YakuTsumo                   // 門前清自摸和
	YakuTanyao                  // 断么九
	YakuPinfu                   // 平和
	YakuIipeikou                // 一盃口
	YakuTon                     // 自風牌 東
	YakuNan                     // 自風牌 南
	YakuXia                     // 自風牌 西
	YakuPei                     // 自風牌 北
	YakuBakazeTon               // 場風牌 東
	YakuBakazeNan               // 場風牌 南
	YakuBakazeXia               // 場風牌 西
	YakuBakazePei               // 場風牌 北
	YakuHaku                    // 白
	YakuHatsu                   // 發
	YakuChun                    // 中
)

type YakuResult struct {
	Yaku YakuType
	Name string
	Han  int
}

var yakuInfo = map[YakuType]struct {
	Name      string
	HanOpen   int
	HanClosed int
}{
	YakuRiichi:    {"立直", 0, 1},
	YakuTsumo:     {"門前清自摸和", 0, 1},
	YakuTanyao:    {"断么九", 1, 1},
	YakuPinfu:     {"平和", 0, 1},
	YakuIipeikou:  {"一盃口", 0, 1},
	YakuTon:       {"自風 東", 1, 1},
	YakuNan:       {"自風 南", 1, 1},
	YakuXia:       {"自風 西", 1, 1},
	YakuPei:       {"自風 北", 1, 1},
	YakuBakazeTon: {"場風 東", 1, 1},
	YakuBakazeNan: {"場風 南", 1, 1},
	YakuBakazeXia: {"場風 西", 1, 1},
	YakuBakazePei: {"場風 北", 1, 1},
	YakuHaku:      {"白", 1, 1},
	YakuHatsu:     {"發", 1, 1},
	YakuChun:      {"中", 1, 1},
}

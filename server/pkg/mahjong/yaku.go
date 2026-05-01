package mahjong

type YakuType int

const (
	YakuRiichi         YakuType = iota // 立直
	YakuTsumo                          // 門前清自摸和
	YakuTanyao                         // 断么九
	YakuPinfu                          // 平和
	YakuIipeikou                       // 一盃口
	YakuTon                            // 自風牌 東
	YakuNan                            // 自風牌 南
	YakuXia                            // 自風牌 西
	YakuPei                            // 自風牌 北
	YakuBakazeTon                      // 場風牌 東
	YakuBakazeNan                      // 場風牌 南
	YakuBakazeXia                      // 場風牌 西
	YakuBakazePei                      // 場風牌 北
	YakuHaku                           // 白
	YakuHatsu                          // 發
	YakuChun                           // 中
	YakuRinshan                        // 嶺上開花
	YakuChankan                        // 搶槓
	YakuHaitei                         // 海底摸月
	YakuHoutei                         // 河底撈魚
	YakuChiitoitsu                     // 七対子
	YakuToitoi                         // 対々和
	YakuSanankou                       // 三暗刻
	YakuSanshokuDoujun                 // 三色同順
	YakuIttsu                          // 一気通貫
	YakuChanta                         // 混全帯么九
	YakuSanshokuDoukou                 // 三色同刻
	YakuShousangen                     // 小三元
	YakuHonroutou                      // 混老頭
	YakuDoubleRiichi                   // ダブル立直
	YakuHonitsu                        // 混一色
	YakuJunchan                        // 純全帯么九
	YakuRyanpeikou                     // 二盃口
	YakuChinitsu                       // 清一色
	YakuKokushi                        // 国士無双
	YakuSuuankou                       // 四暗刻
	YakuDaisangen                      // 大三元
	YakuTsuuiisou                      // 字一色
	YakuRyuuiisou                      // 緑一色
	YakuChinroutou                     // 清老頭
	YakuSuushiihou                     // 四喜和
	YakuChuurenpoutou                  // 九蓮宝燈
	YakuTenhou                         // 天和
	YakuChiihou                        // 地和
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
	YakuRiichi:         {"立直", 0, 1},
	YakuTsumo:          {"门前清自摸和", 0, 1},
	YakuTanyao:         {"断幺九", 1, 1},
	YakuPinfu:          {"平和", 0, 1},
	YakuIipeikou:       {"一杯口", 0, 1},
	YakuTon:            {"自风 東", 1, 1},
	YakuNan:            {"自风 南", 1, 1},
	YakuXia:            {"自风 西", 1, 1},
	YakuPei:            {"自风 北", 1, 1},
	YakuBakazeTon:      {"场风 東", 1, 1},
	YakuBakazeNan:      {"场风 南", 1, 1},
	YakuBakazeXia:      {"场风 西", 1, 1},
	YakuBakazePei:      {"场风 北", 1, 1},
	YakuHaku:           {"白", 1, 1},
	YakuHatsu:          {"發", 1, 1},
	YakuChun:           {"中", 1, 1},
	YakuRinshan:        {"岭上开花", 1, 1},
	YakuChankan:        {"抢杠", 1, 1},
	YakuHaitei:         {"海底摸月", 1, 1},
	YakuHoutei:         {"河底捞鱼", 1, 1},
	YakuChiitoitsu:     {"七对子", 0, 2},
	YakuToitoi:         {"对对和", 2, 2},
	YakuSanankou:       {"三暗刻", 2, 2},
	YakuSanshokuDoujun: {"三色同顺", 1, 2},
	YakuIttsu:          {"一气通贯", 1, 2},
	YakuChanta:         {"混全带幺九", 1, 2},
	YakuSanshokuDoukou: {"三色同刻", 2, 2},
	YakuShousangen:     {"小三元", 2, 2},
	YakuHonroutou:      {"混老头", 2, 2},
	YakuDoubleRiichi:   {"双立直", 0, 2},
	YakuHonitsu:        {"混一色", 2, 3},
	YakuJunchan:        {"纯全带幺九", 2, 3},
	YakuRyanpeikou:     {"二杯口", 0, 3},
	YakuChinitsu:       {"清一色", 5, 6},
	YakuKokushi:        {"国士无双", 0, 13},
	YakuSuuankou:       {"四暗刻", 0, 13},
	YakuDaisangen:      {"大三元", 13, 13},
	YakuTsuuiisou:      {"字一色", 13, 13},
	YakuRyuuiisou:      {"绿一色", 13, 13},
	YakuChinroutou:     {"清老头", 13, 13},
	YakuSuushiihou:     {"四喜和", 13, 13},
	YakuChuurenpoutou:  {"九莲宝灯", 0, 13},
	YakuTenhou:         {"天和", 0, 13},
	YakuChiihou:        {"地和", 0, 13},
}

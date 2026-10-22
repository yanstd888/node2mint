package handler

// Logika murni (pure functions) untuk paket token:
// - perhitungan surcharge (25% configurable, diterapkan SEBELUM multiplier)
// - semantik duration overwrite vs extend
// - reward referral per undangan (1M token)
//
// Semua fungsi di sini tak bergantung DB/state — cukup untuk diuji pure.

// defaultReferralReward: token reward per undangan (inviter + invitee masing-masing).
const defaultReferralReward = 1000000

// referralReward membaca reward dari setting platform (jika ada) atau default.
// Dipanggil oleh handler; di sini hanya fallback konstan.
func referralReward(configured float64) float64 {
	if configured > 0 {
		return configured
	}
	return defaultReferralReward
}

// usageSurcharge: tambahkan 25% (atau rate yang dikonfigurasi) PADA token usage
// SEBELUM dikalikan — sesuai permintaan: "Add 25% token usage surcharge before multiplier".
// `rate` dikonfigurasi dari platform_setting `usage_surcharge_rate` (0.25 = 25%).
// Jika rate <= 0 → tidak ada surcharge (behavior fallback).
func usageSurcharge(tokens, rate float64) float64 {
	if tokens <= 0 || rate <= 0 {
		return tokens
	}
	return tokens * (1 + rate)
}

// effectiveCostWithMultiplier menggabungkan surcharge + multiplier model.
// Surcharge diterapkan pada token usage, multiplier diterapkan setelahnya.
func effectiveCostWithMultiplier(tokens, surchargeRate, multiplier float64) float64 {
	surcharged := usageSurcharge(tokens, surchargeRate)
	if multiplier <= 0 {
		multiplier = 1
	}
	return surcharged * multiplier
}

// PackageDurationSemantik memutuskan apakah pembelian paket baru menimpa atau
// memperpanjang masa berlaku paket yang sudah ada.
//
// Aturan:
//   - Jika ada paket aktif (expires_at > now) dengan sisa token > 0, paket baru
//     EXTEND masa berlaku (expires_at = now + durasi baru) DAN menambah remaining.
//   - Jika tidak ada paket aktif, paket baru OVERWRITE (buat baris baru).
//
// Fungsi ini mengembalikan aksi yang harus dilakukan, bukan menulis DB.
type DurationAction int

const (
	DurationOverwrite DurationAction = iota // tidak ada paket aktif, buat baru
	DurationExtend                          // ada paket aktif, perpanjang + tambah token
)

func packageDurationAction(hasActive bool) DurationAction {
	if hasActive {
		return DurationExtend
	}
	return DurationOverwrite
}

// extendExpiry menghitung expiry baru dari titik waktu yang diberikan.
func extendExpiry(fromUnix int64, durationDays int) int64 {
	return fromUnix + int64(durationDays)*86400
}

// accumulatedExpiry tidak pernah memotong sisa masa aktif: pembelian/reward
// ditambah dari expiry yang lebih akhir, atau dari sekarang jika sudah habis.
func accumulatedExpiry(nowUnix, currentExpiryUnix int64, durationDays int) int64 {
	if currentExpiryUnix > nowUnix {
		nowUnix = currentExpiryUnix
	}
	return extendExpiry(nowUnix, durationDays)
}

// inactivePenalty membatasi denda agar token tidak pernah negatif.
func inactivePenalty(remaining, overdueDays, dailyPenalty int64) int64 {
	if remaining <= 0 || overdueDays <= 0 || dailyPenalty <= 0 {
		return 0
	}
	penalty := overdueDays * dailyPenalty
	if penalty > remaining {
		return remaining
	}
	return penalty
}

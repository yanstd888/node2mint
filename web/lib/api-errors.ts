type ApiErrorLocale = 'en' | 'id' | 'zh' | 'ru' | 'ja';

type ApiErrorMessages = Record<ApiErrorLocale, string>;

const MESSAGES: Record<string, ApiErrorMessages> = {
  invalid_request: {
    en: 'Invalid request.', id: 'Permintaan tidak valid.', zh: '请求无效。', ru: 'Недопустимый запрос.', ja: '無効なリクエストです。',
  },
  unauthorized: {
    en: 'Your session is invalid. Please sign in again.', id: 'Sesi tidak valid. Silakan masuk lagi.', zh: '会话无效，请重新登录。', ru: 'Сессия недействительна. Войдите снова.', ja: 'セッションが無効です。再度ログインしてください。',
  },
  payment_required: {
    en: 'Your balance or credit is insufficient.', id: 'Saldo atau kredit tidak mencukupi.', zh: '余额或额度不足。', ru: 'Недостаточно баланса или кредита.', ja: '残高またはクレジットが不足しています。',
  },
  forbidden: {
    en: 'Access denied.', id: 'Akses ditolak.', zh: '访问被拒绝。', ru: 'Доступ запрещён.', ja: 'アクセスが拒否されました。',
  },
  not_found: {
    en: 'The requested data was not found.', id: 'Data yang diminta tidak ditemukan.', zh: '未找到请求的数据。', ru: 'Запрошенные данные не найдены.', ja: '要求されたデータが見つかりません。',
  },
  method_not_allowed: {
    en: 'This request method is not supported.', id: 'Metode permintaan ini tidak didukung.', zh: '不支持此请求方法。', ru: 'Этот метод запроса не поддерживается.', ja: 'このリクエストメソッドはサポートされていません。',
  },
  conflict: {
    en: 'This action conflicts with the current state.', id: 'Tindakan ini bertentangan dengan kondisi saat ini.', zh: '此操作与当前状态冲突。', ru: 'Действие конфликтует с текущим состоянием.', ja: '現在の状態と競合する操作です。',
  },
  payload_too_large: {
    en: 'The submitted data is too large.', id: 'Data yang dikirim terlalu besar.', zh: '提交的数据过大。', ru: 'Отправленные данные слишком велики.', ja: '送信データが大きすぎます。',
  },
  validation_failed: {
    en: 'Please check the submitted data.', id: 'Periksa data yang dikirim.', zh: '请检查提交的数据。', ru: 'Проверьте отправленные данные.', ja: '送信データを確認してください。',
  },
  rate_limit_exceeded: {
    en: 'Too many requests. Try again later.', id: 'Terlalu banyak permintaan. Coba lagi nanti.', zh: '请求过多，请稍后再试。', ru: 'Слишком много запросов. Повторите позже.', ja: 'リクエストが多すぎます。後でもう一度お試しください。',
  },
  user_rate_limit_exceeded: {
    en: 'Your request limit has been reached. Try again later.', id: 'Batas permintaan akun tercapai. Coba lagi nanti.', zh: '已达到账户请求限制，请稍后再试。', ru: 'Лимит запросов аккаунта достигнут. Повторите позже.', ja: 'アカウントのリクエスト上限に達しました。後でもう一度お試しください。',
  },
  upstream_error: {
    en: 'The upstream service is temporarily unavailable.', id: 'Layanan upstream sedang bermasalah sementara.', zh: '上游服务暂时不可用。', ru: 'Внешний сервис временно недоступен.', ja: '上流サービスは一時的に利用できません。',
  },
  internal_error: {
    en: 'A server error occurred.', id: 'Terjadi kesalahan server.', zh: '服务器发生错误。', ru: 'Произошла ошибка сервера.', ja: 'サーバーエラーが発生しました。',
  },
  request_failed: {
    en: 'The request failed. Check your connection and try again.', id: 'Permintaan gagal. Periksa koneksi lalu coba lagi.', zh: '请求失败，请检查连接后重试。', ru: 'Запрос не выполнен. Проверьте соединение и повторите попытку.', ja: 'リクエストに失敗しました。接続を確認して再試行してください。',
  },
  missing_api_key: {
    en: 'An API key is required.', id: 'API key diperlukan.', zh: '需要 API 密钥。', ru: 'Требуется API-ключ.', ja: 'APIキーが必要です。',
  },
  invalid_api_key: {
    en: 'The API key is invalid.', id: 'API key tidak valid.', zh: 'API 密钥无效。', ru: 'Недействительный API-ключ.', ja: 'APIキーが無効です。',
  },
  key_revoked: {
    en: 'The API key has been revoked.', id: 'API key telah dicabut.', zh: 'API 密钥已撤销。', ru: 'API-ключ отозван.', ja: 'APIキーは取り消されています。',
  },
  key_disabled: {
    en: 'The API key is disabled.', id: 'API key dinonaktifkan.', zh: 'API 密钥已禁用。', ru: 'API-ключ отключён.', ja: 'APIキーは無効化されています。',
  },
  permission_denied: {
    en: 'You do not have permission for this action.', id: 'Anda tidak memiliki izin untuk tindakan ini.', zh: '您无权执行此操作。', ru: 'У вас нет разрешения на это действие.', ja: 'この操作を行う権限がありません。',
  },
  email_invalid: {
    en: 'Enter a valid email address.', id: 'Masukkan alamat email yang valid.', zh: '请输入有效的邮箱地址。', ru: 'Введите корректный email.', ja: '有効なメールアドレスを入力してください。',
  },
  name_invalid: {
    en: 'Enter a valid full name.', id: 'Masukkan nama lengkap yang valid.', zh: '请输入有效的姓名。', ru: 'Введите корректное полное имя.', ja: '有効な氏名を入力してください。',
  },
  password_invalid: {
    en: 'Password must be 8–72 characters.', id: 'Password harus 8–72 karakter.', zh: '密码长度必须为 8–72 个字符。', ru: 'Пароль должен содержать 8–72 символа.', ja: 'パスワードは8～72文字で入力してください。',
  },
  password_mismatch: {
    en: 'Passwords do not match.', id: 'Password tidak sama.', zh: '两次密码不一致。', ru: 'Пароли не совпадают.', ja: 'パスワードが一致しません。',
  },
  invalid_credentials: {
    en: 'Email or password is incorrect.', id: 'Email atau password salah.', zh: '邮箱或密码不正确。', ru: 'Неверный email или пароль.', ja: 'メールアドレスまたはパスワードが正しくありません。',
  },
  email_registered: {
    en: 'This email is already registered.', id: 'Email ini sudah terdaftar.', zh: '此邮箱已注册。', ru: 'Этот email уже зарегистрирован.', ja: 'このメールアドレスは登録済みです。',
  },
  turnstile_failed: {
    en: 'Security verification failed. Try again.', id: 'Verifikasi keamanan gagal. Coba lagi.', zh: '安全验证失败，请重试。', ru: 'Проверка безопасности не пройдена. Повторите попытку.', ja: 'セキュリティ確認に失敗しました。再試行してください。',
  },
  two_factor_required: {
    en: 'Enter your two-factor authentication code.', id: 'Masukkan kode autentikasi dua faktor.', zh: '请输入双因素认证代码。', ru: 'Введите код двухфакторной аутентификации.', ja: '2要素認証コードを入力してください。',
  },
  two_factor_invalid: {
    en: 'The two-factor authentication code is invalid.', id: 'Kode autentikasi dua faktor tidak valid.', zh: '双因素认证代码无效。', ru: 'Неверный код двухфакторной аутентификации.', ja: '2要素認証コードが無効です。',
  },
  account_banned: {
    en: 'This account is blocked.', id: 'Akun ini diblokir.', zh: '此账户已被封禁。', ru: 'Этот аккаунт заблокирован.', ja: 'このアカウントはブロックされています。',
  },
  credit_limit_reached: {
    en: 'The API key credit limit has been reached.', id: 'Batas kredit API key telah tercapai.', zh: '已达到 API 密钥信用额度上限。', ru: 'Достигнут кредитный лимит API-ключа.', ja: 'APIキーのクレジット上限に達しました。',
  },
  insufficient_quota: {
    en: 'The available quota is insufficient.', id: 'Kuota yang tersedia tidak mencukupi.', zh: '可用额度不足。', ru: 'Недостаточно доступной квоты.', ja: '利用可能なクォータが不足しています。',
  },
  origin_not_allowed: {
    en: 'This origin is not allowed.', id: 'Origin ini tidak diizinkan.', zh: '不允许此来源。', ru: 'Этот источник не разрешён.', ja: 'このオリジンは許可されていません。',
  },
  telegram_membership_required: {
    en: 'Telegram membership is required.', id: 'Keanggotaan Telegram diperlukan.', zh: '需要 Telegram 会员资格。', ru: 'Требуется членство в Telegram.', ja: 'Telegramメンバーシップが必要です。',
  },
};

function locale(): ApiErrorLocale {
  if (typeof document === 'undefined') return 'en';
  const value = (document.documentElement.lang || 'en').toLowerCase().split('-')[0];
  return value === 'id' || value === 'zh' || value === 'ru' || value === 'ja' ? value : 'en';
}

export function apiErrorMessage(code: string, fallback: string): string {
  const messages = MESSAGES[code.toLowerCase()];
  return messages?.[locale()] || messages?.en || fallback;
}

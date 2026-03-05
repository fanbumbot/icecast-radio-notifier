START_MSG = \
"""
Вы начали использование бота-рассылки icecast радио
Подробности можете узнать набрав команду /help и
команду /stop, чтобы остановить рассылку
"""

STOP_MSG = \
"Рассылка радио icecast прекращена. Чтобы снова подписаться, введите /start"

HELP_MSG = \
"""
/help - посмотреть команды для управления ботом
/start - начать получать рассылку
/stop - остановить получение рассылки
/notification_status - узнать Ваш статус рассылки
/status - узнать текущий статус icecast радио
/radio_hist - посмотреть историю запуска радио
"""

RADIO_ON_INFO = "Радио icecast в эфире"
RADIO_OFF_INFO = "Радио icecast выключено"

RADIO_ON_NOTIFICATION = "Радио icecast запущено"
RADIO_OFF_NOTIFICATION = "Радио icecast остановлено"

IS_SUBSCRIBED = "Вы подписаны на рассылку"
IS_NOT_SUBSCRIBED = "Вы не подписаны на рассылку"

BROADCAST_HIST_INIT = "История icecast радио:"
BROADCAST_HIST = "Эфир от [%s] завершился [%s]"
BROADCAST_HIST_NOW = "В данный момент идёт эфир от %s"
NO_BROADCASTS_HIST = "История icecast радио ещё не началась"
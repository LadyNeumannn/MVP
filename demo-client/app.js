// Просто ручной инструмент для проверки сценариев Client API

const API_BASE = window.API_BASE || 'http://localhost:8080';

const state = {
  restaurants: [],
  selectedRestaurantId: null,
  menu: [],
  cart: new Map(), 
  pollTimer: null,
};

const el = {
  restaurantSearch: document.getElementById('restaurant-search'),
  restaurantSelect: document.getElementById('restaurant-select'),
  refreshRestaurants: document.getElementById('refresh-restaurants'),
  menuSearch: document.getElementById('menu-search'),
  menuList: document.getElementById('menu-list'),
  cartList: document.getElementById('cart-list'),
  cartTotal: document.getElementById('cart-total'),
  customerName: document.getElementById('customer-name'),
  customerPhone: document.getElementById('customer-phone'),
  deliveryAddress: document.getElementById('delivery-address'),
  placeOrderBtn: document.getElementById('place-order'),
  orderError: document.getElementById('order-error'),
  orderStatus: document.getElementById('order-status'),
};

function money(cents) {
  return (cents / 100).toFixed(2) + ' \u20BD';
}

async function api(path, options = {}) {
  const res = await fetch(API_BASE + path, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...(options.headers || {}) },
  });
  const text = await res.text();
  const data = text ? JSON.parse(text) : null;
  if (!res.ok) {
    const message = (data && data.error) ? data.error : `HTTP ${res.status}`;
    throw new Error(message);
  }
  return data;
}

async function loadRestaurants() {
  const q = el.restaurantSearch.value.trim();
  el.restaurantSelect.innerHTML = '<option value="">— загрузка... —</option>';
  try {
    const qs = q ? `?q=${encodeURIComponent(q)}` : '';
    state.restaurants = await api(`/api/v1/restaurants${qs}`);
  } catch (err) {
    el.restaurantSelect.innerHTML = '<option value="">— ошибка загрузки —</option>';
    console.error(err);
    return;
  }

  el.restaurantSelect.innerHTML = '<option value="">— выберите заведение —</option>';
  for (const r of state.restaurants) {
    const opt = document.createElement('option');
    opt.value = r.id;
    opt.textContent = r.name;
    el.restaurantSelect.appendChild(opt);
  }
}

async function loadMenu(restaurantId) {
  state.menu = [];
  renderMenu();
  el.menuSearch.disabled = !restaurantId;
  if (!restaurantId) return;

  try {
    const q = el.menuSearch.value.trim();
    const qs = q ? `?q=${encodeURIComponent(q)}` : '';
    state.menu = await api(`/api/v1/restaurants/${restaurantId}/menu${qs}`);
  } catch (err) {
    el.menuList.innerHTML = `<li class="empty">Ошибка загрузки меню: ${err.message}</li>`;
    return;
  }
  renderMenu();
}

function renderMenu() {
  el.menuList.innerHTML = '';
  if (state.menu.length === 0) {
    el.menuList.innerHTML = '<li class="empty">Меню пусто или заведение не выбрано</li>';
    return;
  }
  for (const item of state.menu) {
    const li = document.createElement('li');
    if (!item.available) li.classList.add('unavailable');

    const label = document.createElement('span');
    label.textContent = `${item.name} — ${money(item.price_cents)}`;
    li.appendChild(label);

    const btn = document.createElement('button');
    btn.textContent = item.available ? '+ в корзину' : 'нет в наличии';
    btn.style.width = 'auto';
    btn.disabled = !item.available;
    btn.addEventListener('click', () => addToCart(item));
    li.appendChild(btn);

    el.menuList.appendChild(li);
  }
}

function addToCart(item) {
  const entry = state.cart.get(item.id) || { item, qty: 0 };
  entry.qty += 1;
  state.cart.set(item.id, entry);
  renderCart();
}

function changeQty(itemId, delta) {
  const entry = state.cart.get(itemId);
  if (!entry) return;
  entry.qty += delta;
  if (entry.qty <= 0) {
    state.cart.delete(itemId);
  }
  renderCart();
}

function renderCart() {
  el.cartList.innerHTML = '';
  let total = 0;

  if (state.cart.size === 0) {
    el.cartList.innerHTML = '<li class="empty">Корзина пуста</li>';
  }

  for (const { item, qty } of state.cart.values()) {
    total += item.price_cents * qty;

    const li = document.createElement('li');
    const label = document.createElement('span');
    label.textContent = `${item.name} × ${qty}`;
    li.appendChild(label);

    const controls = document.createElement('span');
    controls.className = 'qty-controls';

    const minus = document.createElement('button');
    minus.textContent = '−';
    minus.addEventListener('click', () => changeQty(item.id, -1));
    controls.appendChild(minus);

    const plus = document.createElement('button');
    plus.textContent = '+';
    plus.addEventListener('click', () => changeQty(item.id, +1));
    controls.appendChild(plus);

    li.appendChild(controls);
    el.cartList.appendChild(li);
  }

  el.cartTotal.textContent = money(total);
  el.placeOrderBtn.disabled = state.cart.size === 0;
}

async function placeOrder() {
  el.orderError.textContent = '';
  const customerName = el.customerName.value.trim();
  const customerPhone = el.customerPhone.value.trim();
  const deliveryAddress = el.deliveryAddress.value.trim();
  if (!customerName) {
    el.orderError.textContent = 'Укажите имя покупателя';
    return;
  }
  if (!customerPhone) {
    el.orderError.textContent = 'Укажите телефон';
    return;
  }
  if (!deliveryAddress) {
    el.orderError.textContent = 'Укажите адрес доставки';
    return;
  }
  if (!state.selectedRestaurantId) {
    el.orderError.textContent = 'Выберите заведение';
    return;
  }

  const items = Array.from(state.cart.values()).map(({ item, qty }) => ({
    menu_item_id: item.id,
    quantity: qty,
  }));

  const idempotencyKey = crypto.randomUUID();

  el.placeOrderBtn.disabled = true;
  try {
    const order = await api('/api/v1/orders', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify({
        restaurant_id: state.selectedRestaurantId,
        customer_name: customerName,
        customer_phone: customerPhone,
        delivery_address: deliveryAddress,
        items,
      }),
    });
    state.cart.clear();
    renderCart();
    startPolling(order.id);
  } catch (err) {
    el.orderError.textContent = err.message;
  } finally {
    el.placeOrderBtn.disabled = state.cart.size === 0;
  }
}

function startPolling(orderId) {
  if (state.pollTimer) clearInterval(state.pollTimer);

  const tick = async () => {
    try {
      const order = await api(`/api/v1/orders/${orderId}`);
      renderOrderStatus(order);
      if (order.status === 'delivered' || order.status === 'cancelled') {
        clearInterval(state.pollTimer);
      }
    } catch (err) {
      console.error(err);
    }
  };

  tick();
  state.pollTimer = setInterval(tick, 3000);
}

function renderOrderStatus(order) {
  const itemsHtml = order.items.map(
    (i) => `<li>${i.name_snapshot} × ${i.quantity} — ${money(i.price_cents_snapshot * i.quantity)}</li>`
  ).join('');
  const badgeClass = order.status === 'cancelled' ? 'status-badge status-badge--cancelled' : 'status-badge';

  el.orderStatus.innerHTML = `
    <p>Заказ <code>${order.id}</code></p>
    <p>Статус: <span class="${badgeClass}">${order.status}</span></p>
    <p>Адрес доставки: ${order.delivery_address}</p>
    <ul>${itemsHtml}</ul>
    <p>Итого: <strong>${money(order.total_cents)}</strong></p>
    <p class="empty">Статус обновляется автоматически каждые 3 сек.
    Меняйте его через Partner API у заведения. Около 15% заказов
    restaurant-example отменяет сам — так проверяется ветка cancelled.</p>
  `;
}

el.restaurantSelect.addEventListener('change', (e) => {
  state.selectedRestaurantId = e.target.value || null;
  state.cart.clear();
  renderCart();
  loadMenu(state.selectedRestaurantId);
});
el.refreshRestaurants.addEventListener('click', loadRestaurants);
el.restaurantSearch.addEventListener('change', loadRestaurants);
el.menuSearch.addEventListener('change', () => loadMenu(state.selectedRestaurantId));
el.placeOrderBtn.addEventListener('click', placeOrder);

loadRestaurants();
renderCart();

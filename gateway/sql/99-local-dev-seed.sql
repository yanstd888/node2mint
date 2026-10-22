-- Local development only. This file is mounted only by docker-compose.yml.
-- Never mount it in a deployed environment.

INSERT INTO "user" (email, name, role, wallet_balance, password_hash)
VALUES
  ('admin@test.local', 'Local Admin', 'ADMIN', 50000.00, '$2a$10$9VA7vVYg/spL9bGPt5bNu.Q9XWqpG0T52TOG5kwu0Vz3ySyYFvGKO'),
  ('seller@test.local', 'Local Seller', 'SELLER', 10000.00, '$2a$10$9VA7vVYg/spL9bGPt5bNu.Q9XWqpG0T52TOG5kwu0Vz3ySyYFvGKO')
ON CONFLICT (email) DO UPDATE SET
  password_hash = EXCLUDED.password_hash,
  role          = EXCLUDED.role,
  name          = EXCLUDED.name,
  wallet_balance = GREATEST("user".wallet_balance, EXCLUDED.wallet_balance);

-- Toko dev untuk seller@test.local; logo via media upload (dev only).
INSERT INTO market_store (user_id, name, description, is_active, wallet_balance)
SELECT u.id, 'Demo Store', 'Demo seller store for local development.', TRUE, 0
FROM "user" u WHERE u.email = 'seller@test.local'
ON CONFLICT (user_id) DO UPDATE SET name = EXCLUDED.name, description = EXCLUDED.description, is_active = TRUE;

-- Dev seed: 10+3 produk dummy, langsung aktif, terhubung ke store seller@test.local.
WITH seeded AS (
  SELECT u.id AS seller_id, s.id AS store_id, v.*
  FROM "user" u
  JOIN market_store s ON s.user_id = u.id
  CROSS JOIN (
    VALUES
      ('AI Art Generator', 'Local demo product.', 25000.00, 99, 'https://picsum.photos/seed/art/400/300'),
      ('SEO Writer Pro', 'Local demo product.', 35000.00, 99, 'https://picsum.photos/seed/seo/400/300'),
      ('Code Review Bot', 'Local demo product.', 45000.00, 50, 'https://picsum.photos/seed/code/400/300'),
      ('Marketplace Dummy 1', 'Auto-added item.', 1000.00, 100, 'https://picsum.photos/seed/d1/400/300'),
      ('Marketplace Dummy 2', 'Auto-added item.', 1200.00, 100, 'https://picsum.photos/seed/d2/400/300'),
      ('Marketplace Dummy 3', 'Auto-added item.', 1400.00, 100, 'https://picsum.photos/seed/d3/400/300'),
      ('Marketplace Dummy 4', 'Auto-added item.', 1600.00, 100, 'https://picsum.photos/seed/d4/400/300'),
      ('Marketplace Dummy 5', 'Auto-added item.', 1800.00, 100, 'https://picsum.photos/seed/d5/400/300'),
      ('Marketplace Dummy 6', 'Auto-added item.', 2000.00, 100, 'https://picsum.photos/seed/d6/400/300'),
      ('Marketplace Dummy 7', 'Auto-added item.', 2200.00, 100, 'https://picsum.photos/seed/d7/400/300'),
      ('Marketplace Dummy 8', 'Auto-added item.', 2400.00, 100, 'https://picsum.photos/seed/d8/400/300'),
      ('Marketplace Dummy 9', 'Auto-added item.', 2600.00, 100, 'https://picsum.photos/seed/d9/400/300'),
      ('Marketplace Dummy 10', 'Auto-added item.', 2800.00, 100, 'https://picsum.photos/seed/d10/400/300')
  ) AS v(title, description, price, stock, image_url)
  WHERE u.email = 'seller@test.local'
)
INSERT INTO market_product (seller_id, store_id, title, description, price, stock, image_url, is_active)
SELECT seller_id, store_id, title, description, price, stock, image_url, TRUE
FROM seeded
WHERE NOT EXISTS (SELECT 1 FROM market_product mp WHERE mp.title = seeded.title);

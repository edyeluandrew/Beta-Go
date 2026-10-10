-- Clear dish photos for the walkthrough kitchen (chips, chicken, pork, burgers, pizza).
-- No-op if that kitchen is not in this database.

INSERT INTO menu_categories (vendor_id, name)
SELECT v.id, c.name
FROM vendors v
CROSS JOIN (VALUES
    ('Chips'),
    ('Chicken'),
    ('Pork'),
    ('Burgers'),
    ('Pizza')
) AS c(name)
WHERE lower(v.contact_email) = 'anonymousdev256@gmail.com'
  AND NOT EXISTS (
    SELECT 1
    FROM menu_categories mc
    WHERE mc.vendor_id = v.id
      AND lower(mc.name) = lower(c.name)
  );

UPDATE menu_items mi
SET name = 'Fried chicken'
FROM vendors v
WHERE mi.vendor_id = v.id
  AND lower(v.contact_email) = 'anonymousdev256@gmail.com'
  AND mi.name = 'Chicken pieces';

WITH kitchen AS (
    SELECT id
    FROM vendors
    WHERE lower(contact_email) = 'anonymousdev256@gmail.com'
),
wanted (category, name, description, price, photo) AS (
    VALUES
        ('Chips', 'Chips', 'Crispy golden chips.', 5000, 'https://upload.wikimedia.org/wikipedia/commons/4/42/French_fries.jpg'),
        ('Chicken', 'Fried chicken', 'Crunchy fried chicken pieces.', 15000, 'https://upload.wikimedia.org/wikipedia/commons/b/b7/Fried_chicken.jpg'),
        ('Pork', 'Pork chops', 'Grilled pork chops.', 18000, 'https://upload.wikimedia.org/wikipedia/commons/0/00/Pork_chop.jpg'),
        ('Burgers', 'Beef burger', 'Beef burger with chips on the side.', 12000, 'https://upload.wikimedia.org/wikipedia/commons/6/62/Bacon_Cheeseburger_with_French_Fries_on_plate.JPG'),
        ('Pizza', 'Chicken pizza', 'Hot pizza, cut and ready.', 20000, 'https://upload.wikimedia.org/wikipedia/commons/9/91/Pizza-3007395.jpg'),
        ('Chicken', 'Chicken and chips', 'Fried chicken with a pile of chips.', 18000, 'https://upload.wikimedia.org/wikipedia/commons/4/40/KFC_Chicken_and_chips.jpg'),
        ('Chicken', 'Rolex', 'Chapati rolled with egg. Street food.', 6000, 'https://upload.wikimedia.org/wikipedia/commons/5/5b/Chapati.jpg')
)
UPDATE menu_items mi
SET
    description = w.description,
    price_ugx = w.price,
    available = true,
    photo_url = w.photo,
    category_id = mc.id,
    updated_at = now()
FROM kitchen k
JOIN wanted w ON true
JOIN menu_categories mc ON mc.vendor_id = k.id AND lower(mc.name) = lower(w.category)
WHERE mi.vendor_id = k.id
  AND lower(mi.name) = lower(w.name);

INSERT INTO menu_items (vendor_id, category_id, name, description, price_ugx, available, photo_url)
SELECT k.id, mc.id, w.name, w.description, w.price, true, w.photo
FROM vendors k
CROSS JOIN (
    VALUES
        ('Chips', 'Chips', 'Crispy golden chips.', 5000, 'https://upload.wikimedia.org/wikipedia/commons/4/42/French_fries.jpg'),
        ('Chicken', 'Fried chicken', 'Crunchy fried chicken pieces.', 15000, 'https://upload.wikimedia.org/wikipedia/commons/b/b7/Fried_chicken.jpg'),
        ('Pork', 'Pork chops', 'Grilled pork chops.', 18000, 'https://upload.wikimedia.org/wikipedia/commons/0/00/Pork_chop.jpg'),
        ('Burgers', 'Beef burger', 'Beef burger with chips on the side.', 12000, 'https://upload.wikimedia.org/wikipedia/commons/6/62/Bacon_Cheeseburger_with_French_Fries_on_plate.JPG'),
        ('Pizza', 'Chicken pizza', 'Hot pizza, cut and ready.', 20000, 'https://upload.wikimedia.org/wikipedia/commons/9/91/Pizza-3007395.jpg'),
        ('Chicken', 'Chicken and chips', 'Fried chicken with a pile of chips.', 18000, 'https://upload.wikimedia.org/wikipedia/commons/4/40/KFC_Chicken_and_chips.jpg'),
        ('Chicken', 'Rolex', 'Chapati rolled with egg. Street food.', 6000, 'https://upload.wikimedia.org/wikipedia/commons/5/5b/Chapati.jpg')
) AS w(category, name, description, price, photo)
JOIN menu_categories mc ON mc.vendor_id = k.id AND lower(mc.name) = lower(w.category)
WHERE lower(k.contact_email) = 'anonymousdev256@gmail.com'
  AND NOT EXISTS (
    SELECT 1
    FROM menu_items mi
    WHERE mi.vendor_id = k.id
      AND lower(mi.name) = lower(w.name)
  );

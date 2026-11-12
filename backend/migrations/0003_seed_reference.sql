-- 0003: canonical sport catalogue.
--
-- These rows are real reference data defined by the product (master plan
-- section 4), not sample data. They are the only rows inserted by migrations.

INSERT INTO sports (name, slug, display_order) VALUES
    ('Football',        'football',        1),
    ('Basketball',      'basketball',      2),
    ('American Football', 'american-football', 3),
    ('Tennis',          'tennis',          4),
    ('Golf',            'golf',            5)
ON CONFLICT (slug) DO NOTHING;

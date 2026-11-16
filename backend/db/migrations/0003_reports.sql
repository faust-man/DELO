CREATE OR REPLACE VIEW finance_summary AS
SELECT o.id AS org,c.currency,
coalesce((SELECT sum(p.amount) FROM payments p WHERE p.org=o.id AND p.currency=c.currency),0) AS received,
coalesce((SELECT sum(r.amount) FROM refunds r JOIN payments p ON p.org=r.org AND p.id=r.payment_id WHERE r.org=o.id AND p.currency=c.currency),0) AS refunded,
coalesce((SELECT sum(ch.amount) FROM charges ch WHERE ch.org=o.id AND ch.currency=c.currency),0) AS charged,
coalesce((SELECT sum(a.amount) FROM allocations a JOIN payments p ON p.org=a.org AND p.id=a.payment_id WHERE a.org=o.id AND p.currency=c.currency),0) AS allocated,
c.currency AS id
FROM organizations o CROSS JOIN LATERAL (SELECT currency FROM payments WHERE org=o.id UNION SELECT currency FROM charges WHERE org=o.id) c;

# คู่มือการตั้งค่า Alert Rule & Webhook Connector ใน Kibana UI

คู่มือนี้อธิบายวิธีตั้งค่า Kibana Alert Rule เพื่อให้ระบบตรวจจับ Error Log และยิง Webhook ส่งมาที่ AI RCA Agent โดยทำผ่าน Kibana UI ได้เองอย่างปลอดภัย ไม่ต้องใช้ Script ใดๆ

---

## ส่วนที่ 1: สร้าง Webhook Connector (เชื่อมต่อไปหา AI Agent)

1. เปิดเบราว์เซอร์ไปที่ **Kibana**: `http://localhost:5601`
2. เมนูด้านซ้ายล่าง คลิก **Stack Management**
3. ภายใต้หัวข้อ **Alerts and Insights** $\rightarrow$ คลิก **Connectors**
4. คลิกปุ่ม **Create connector** (มุมขวาบน)
5. เลือกประเภท connector เป็น **Webhook**
6. กรอกรายละเอียดดังนี้:
   - **Name:** `RCA Agent Webhook`
   - **Method:** `POST`
   - **URL:** `http://rca-agent:8080/webhook/alert` *(หรือ `http://localhost:8080/webhook/alert` หากรันนอก Docker)*
   - **Authentication:** `None`
7. คลิกปุ่ม **Save & close**

---

## ส่วนที่ 2: สร้าง Alert Rule (กำหนดเงื่อนไขการตรวจจับ)

1. ที่เมนู **Stack Management** $\rightarrow$ คลิก **Rules**
2. คลิกปุ่ม **Create rule**
3. กรอกรายละเอียดทั่วไป:
   - **Name:** `High 5xx Spike on App`
   - **Check every:** `1 minute`
   - **Notify:** `On check`
4. เลือกเงื่อนไขตรวจจับ (**Rule type**):
   - เลือก **Elasticsearch query**
   - **Index:** `logs-app-*`
   - **Query (KQL):**
     ```text
     level: "ERROR" or http_status >= 500
     ```
   - **Threshold:** `IS ABOVE 3` ในช่วง `1 minute`  
     *(หมายถึง: หากมี log ที่เป็น ERROR หรือสถานะ 500 ขึ้นไป เกิน 3 รายการใน 1 นาที ให้แจ้งเตือนทันที)*
5. กำหนดการทำงานเมื่อเข้าเงื่อนไข (**Actions**):
   - ในหัวข้อ Actions เลือก Connector ที่สร้างไว้: `RCA Agent Webhook`
   - เลือก Action group: **Query matched**
   - ในช่อง **Body (JSON)** ให้ระบุ payload template ดังนี้:
     ```json
     {
       "rule_id": "{{{rule.id}}}",
       "rule_name": "{{{rule.name}}}",
       "timestamp": "{{{date}}}",
       "service": "order-service",
       "alert_reason": "High error rate detected in the last minute"
     }
     ```
6. คลิก **Save**

---

## การทำงานหลังจากตั้งค่าเสร็จ
- Kibana จะคอยตรวจสอบ Index `logs-app-*` ทุกๆ 1 นาที
- เมื่อมีการยิง Endpoint ที่ทำให้เกิด Error เกิน 3 ครั้ง Kibana จะส่ง HTTP POST เข้าหา AI RCA Agent ทันที
- เราสามารถกดเปิด/ปิด (Enable/Disable) Rule ได้จากหน้าจอนี้ตลอดเวลาตามต้องการ

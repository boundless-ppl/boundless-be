# Dream Tracker API Contract

Base path: `/dream-trackers`

All Dream Tracker endpoints require authentication.

Header:

```http
Authorization: Bearer <access_token>
```

## Tujuan Kontrak

Halaman Dream Tracker ini sebaiknya dibangun dengan 3 read API utama:

1. API ringkasan kartu atas
2. API list by beasiswa dan universitas
3. API detail by dream tracker ID

Supaya initial load tetap efisien:

- FE bisa panggil `GET /dream-trackers/summary` dan `GET /dream-trackers/grouped` secara paralel.
- `GET /dream-trackers/grouped` bisa memakai `include_default_detail=true` agar panel kanan langsung dapat 1 detail default tanpa extra loading state yang terasa pecah.
- Setelah user pindah pilihan universitas/beasiswa, FE cukup panggil `GET /dream-trackers/:id`.

Untuk dokumen:

- Upload harus idempotent untuk dokumen yang sama.
- Jika user sudah punya dokumen valid, misalnya KTP yang sudah `VERIFIED`, backend harus `reuse` dokumen lama itu.
- Jika dokumen lama dipakai ulang, backend tidak upload ulang dan tidak trigger AI verification lagi.
- Setelah upload atau reuse, response harus langsung membawa `ai_message` untuk ditampilkan di bawah item dokumen.

## Common Error Response

```json
{
  "error": "invalid input"
}
```

Common error messages:

- `authentication failed`
- `invalid input`
- `invalid id format`
- `dream tracker not found`
- `dream requirement not found`
- `document not found`
- `internal server error`

## Enums

### Dream Tracker Status

- `ACTIVE`
- `COMPLETED`
- `ARCHIVED`

### Dream Requirement Status

- `NOT_UPLOADED`
- `UPLOADED`
- `REVIEWING`
- `VERIFIED`
- `REJECTED`
- `REUSED`

### Review Source

- `NEW_UPLOAD`
- `REUSED_EXISTING`
- `SKIPPED_ALREADY_VERIFIED`

### Review Status

- `NOT_STARTED`
- `PENDING`
- `PROCESSING`
- `COMPLETED`
- `FAILED`
- `SKIPPED`

## 1. POST `/dream-trackers`

Dipakai untuk menyimpan pilihan user ke Dream Tracker.

Flow yang didukung:

- create manual dari browse/list program
- create dari hasil AI recommendation

Content type:

```http
Content-Type: application/json
```

Request body:

- `program_id`: `string`, required
- `admission_id`: `UUID`, optional
- `funding_id`: `UUID`, optional
- `title`: `string`, required
- `status`: `Dream Tracker Status`, optional, default `ACTIVE`
- `source_type`: `string`, required
- `req_submission_id`: `UUID`, optional
- `source_rec_result_id`: `UUID`, optional

Manual example:

```json
{
  "program_id": "program-1",
  "admission_id": "admission-1",
  "funding_id": "funding-1",
  "title": "University of Bristol",
  "source_type": "MANUAL"
}
```

Recommendation example:

```json
{
  "program_id": "program-1",
  "admission_id": "admission-1",
  "funding_id": "funding-1",
  "title": "University of Bristol",
  "source_type": "RECOMMENDATION",
  "req_submission_id": "submission-1",
  "source_rec_result_id": "result-1"
}
```

Notes:

- `source_type=RECOMMENDATION` dipakai saat tracker dibuat dari hasil AI recommendation CV/transcript/profile.
- `req_submission_id` berguna untuk tahu recommendation submission asal tracker.
- `source_rec_result_id` bersifat optional. Field ini dipakai jika backend/analytics perlu tahu recommendation item mana yang dipilih user.
- Jika FE belum punya `source_rec_result_id`, tracker tetap boleh dibuat selama data minimum valid.

Success response: `201 Created`

```json
{
  "dream_tracker_id": "tracker-1",
  "status": "ACTIVE"
}
```

Possible responses:

- `201 Created`
- `400 Bad Request`
- `401 Unauthorized`
- `500 Internal Server Error`

## 2. GET `/dream-trackers/summary`

Dipakai untuk kartu bagian atas:

- total aplikasi
- belum lengkap
- selesai
- deadline mendekat

Success response: `200 OK`

```json
{
  "total_applications": 4,
  "incomplete_count": 4,
  "completed_count": 0,
  "deadline_near_count": 1
}
```

Possible responses:

- `200 OK`
- `401 Unauthorized`
- `500 Internal Server Error`

## 3. GET `/dream-trackers/grouped`

Dipakai untuk panel kiri: list universitas dan beasiswa.

Query params:

- `include_default_detail`: `boolean`, optional, default `false`
- `selected_dream_tracker_id`: `UUID`, optional

Jika `include_default_detail=true`, backend mengembalikan 1 detail default supaya initial render tidak perlu nunggu API ketiga.

Success response: `200 OK`

```json
{
  "default_selected_dream_tracker_id": "tracker-1",
  "universities": [
    {
      "university_id": "univ-1",
      "university_name": "University of Bristol",
      "items": [
        {
          "dream_tracker_id": "tracker-1",
          "title": "University of Bristol",
          "program_name": "MSc Computer Science",
          "admission_name": "Fall 2027",
          "status": "ACTIVE",
          "status_label": "Sedang Diproses",
          "completion_percentage": 33,
          "is_selected": true
        }
      ]
    }
  ],
  "fundings": [
    {
      "funding_id": "funding-1",
      "funding_name": "LPDP Scholarship",
      "items": [
        {
          "dream_tracker_id": "tracker-1",
          "title": "University of Bristol",
          "program_name": "MSc Computer Science",
          "university_name": "University of Bristol",
          "status": "ACTIVE",
          "status_label": "Sedang Diproses",
          "completion_percentage": 33,
          "is_selected": true
        }
      ]
    }
  ],
  "default_detail": {
    "dream_tracker_id": "tracker-1",
    "title": "University of Bristol",
    "subtitle": "MSc Computer Science",
    "status": "ACTIVE"
  }
}
```

Notes:

- `default_detail` hanya muncul jika `include_default_detail=true`.
- Bentuk `default_detail` boleh berupa payload penuh yang sama persis dengan `GET /dream-trackers/:id`.
- Endpoint ini adalah sumber utama untuk navigasi kiri, bukan untuk detail lengkap semua tracker.

Possible responses:

- `200 OK`
- `400 Bad Request`
- `401 Unauthorized`
- `500 Internal Server Error`

## 4. GET `/dream-trackers/:id`

Dipakai untuk panel kanan: header universitas, timeline, requirements, funding, dan AI message pada dokumen.

Path params:

- `id`: UUID, required

Success response: `200 OK`

```json
{
  "dream_tracker_id": "tracker-1",
  "title": "University of Bristol",
  "subtitle": "MSc Computer Science",
  "status": "ACTIVE",
  "status_label": "Sedang Diproses",
  "status_variant": "IN_PROGRESS",
  "created_at": "2026-04-07T08:00:00Z",
  "updated_at": "2026-04-07T08:00:00Z",
  "deadline_at": "2026-08-01T00:00:00Z",
  "summary": {
    "completion_percentage": 33,
    "completed_requirements": 1,
    "total_requirements": 3,
    "next_deadline_at": "2026-08-01T00:00:00Z",
    "is_deadline_near": false,
    "is_overdue": false
  },
  "program": {
    "program_id": "program-1",
    "program_name": "MSc Computer Science",
    "university_name": "University of Bristol",
    "admission_name": "Fall 2027",
    "intake": "Fall 2027",
    "admission_url": "https://example.edu/apply",
    "admission_deadline": "2026-08-01T00:00:00Z"
  },
  "milestones": [
    {
      "dream_milestone_id": "milestone-1",
      "title": "Pendaftaran Dibuka",
      "status": "DONE",
      "deadline_date": "2026-04-01T00:00:00Z"
    },
    {
      "dream_milestone_id": "milestone-2",
      "title": "Batas Pendaftaran",
      "status": "NOT_STARTED",
      "deadline_date": "2026-08-01T00:00:00Z"
    }
  ],
  "requirements": [
    {
      "dream_req_status_id": "req-ktp-1",
      "req_catalog_id": "catalog-ktp",
      "requirement_key": "ktp",
      "requirement_label": "KTP",
      "category": "IDENTITY",
      "status": "REUSED",
      "status_label": "Sudah tersedia",
      "status_variant": "SUCCESS",
      "can_upload": false,
      "needs_reupload": false,
      "document": {
        "document_id": "doc-ktp-1",
        "document_type": "KTP",
        "original_filename": "ktp.pdf",
        "public_url": "https://storage.googleapis.com/bucket/ktp.pdf",
        "uploaded_at": "2026-04-06T08:00:00Z"
      },
      "review": {
        "source": "REUSED_EXISTING",
        "status": "SKIPPED",
        "is_reused": true,
        "is_already_verified": true,
        "ai_message": "KTP sudah pernah diverifikasi, jadi dokumen lama dipakai kembali.",
        "last_processed_at": "2026-04-06T08:05:00Z"
      }
    },
    {
      "dream_req_status_id": "req-transcript-1",
      "req_catalog_id": "catalog-transcript",
      "requirement_key": "transcript",
      "requirement_label": "Transkrip Nilai",
      "category": "ACADEMIC",
      "status": "NOT_UPLOADED",
      "status_label": "Belum diunggah",
      "status_variant": "DEFAULT",
      "can_upload": true,
      "needs_reupload": false,
      "document": null,
      "review": {
        "source": "NEW_UPLOAD",
        "status": "NOT_STARTED",
        "is_reused": false,
        "is_already_verified": false,
        "ai_message": null,
        "last_processed_at": null
      }
    }
  ],
  "fundings": [
    {
      "funding_id": "funding-1",
      "nama_beasiswa": "Chevening",
      "provider": "Chevening",
      "status": "SELECTED"
    }
  ]
}
```

Notes:

- `requirements[].review.ai_message` adalah text yang bisa langsung ditampilkan di bawah komponen upload.
- Jika dokumen lama dipakai ulang, UI tetap bisa menampilkan pesan AI tanpa menjalankan review lagi.
- `document` boleh `null` jika requirement belum punya dokumen.

Possible responses:

- `200 OK`
- `400 Bad Request`
- `401 Unauthorized`
- `404 Not Found`
- `500 Internal Server Error`

## Upload / Reuse Dokumen

Read API utamanya tetap 3, tapi untuk action upload dibutuhkan endpoint berikut.

## POST `/dream-trackers/requirements/:id/document`

Upload dokumen requirement, atau otomatis reuse dokumen lama yang sudah valid.

Path params:

- `id`: UUID, required, dream requirement status ID

Content type:

```http
Content-Type: multipart/form-data
```

Form fields:

- `file`: file, optional jika backend menemukan dokumen reusable
- `document_type`: `string`, required
- `reuse_if_exists`: `boolean`, optional, default `true`

Behavior:

- Backend cek dulu apakah user sudah punya dokumen `VERIFIED` dengan `document_type` yang sama.
- Jika ada dan `reuse_if_exists=true`, backend attach dokumen lama ke requirement.
- Jika tidak ada, backend upload file baru lalu jalankan review.
- Jika requirement sudah terhubung ke dokumen `VERIFIED` yang sama, backend return sukses tanpa proses ulang.

Success response saat reuse existing document: `200 OK`

```json
{
  "dream_req_status_id": "req-ktp-1",
  "status": "REUSED",
  "status_label": "Sudah tersedia",
  "status_variant": "SUCCESS",
  "document": {
    "document_id": "doc-ktp-1",
    "document_type": "KTP",
    "original_filename": "ktp.pdf",
    "public_url": "https://storage.googleapis.com/bucket/ktp.pdf",
    "uploaded_at": "2026-04-06T08:00:00Z"
  },
  "review": {
    "source": "REUSED_EXISTING",
    "status": "SKIPPED",
    "is_reused": true,
    "is_already_verified": true,
    "ai_message": "KTP sudah pernah diverifikasi, jadi tidak perlu upload dan verifikasi ulang.",
    "last_processed_at": "2026-04-07T08:00:00Z"
  }
}
```

Success response saat upload baru dan review selesai: `201 Created`

```json
{
  "dream_req_status_id": "req-transcript-1",
  "status": "VERIFIED",
  "status_label": "Berhasil diunggah",
  "status_variant": "SUCCESS",
  "document": {
    "document_id": "doc-transcript-1",
    "document_type": "TRANSCRIPT",
    "original_filename": "transcript.pdf",
    "public_url": "https://storage.googleapis.com/bucket/transcript.pdf",
    "uploaded_at": "2026-04-07T08:00:00Z"
  },
  "review": {
    "source": "NEW_UPLOAD",
    "status": "COMPLETED",
    "is_reused": false,
    "is_already_verified": false,
    "ai_message": "Dokumen valid dan sesuai dengan requirement Transkrip Nilai.",
    "last_processed_at": "2026-04-07T08:00:04Z"
  }
}
```

Success response saat upload baru tapi review masih jalan: `202 Accepted`

```json
{
  "dream_req_status_id": "req-transcript-1",
  "status": "REVIEWING",
  "status_label": "Sedang diperiksa",
  "status_variant": "IN_PROGRESS",
  "document": {
    "document_id": "doc-transcript-2",
    "document_type": "TRANSCRIPT",
    "original_filename": "transcript_v2.pdf",
    "public_url": "https://storage.googleapis.com/bucket/transcript_v2.pdf",
    "uploaded_at": "2026-04-07T08:00:00Z"
  },
  "review": {
    "source": "NEW_UPLOAD",
    "status": "PROCESSING",
    "is_reused": false,
    "is_already_verified": false,
    "ai_message": "Dokumen berhasil diunggah dan sedang diperiksa AI.",
    "last_processed_at": "2026-04-07T08:00:01Z"
  }
}
```

Possible responses:

- `200 OK`
- `201 Created`
- `202 Accepted`
- `400 Bad Request`
- `401 Unauthorized`
- `404 Not Found`
- `500 Internal Server Error`

## Rekomendasi Implementasi Backend

- Optimalkan `GET /dream-trackers/grouped` supaya tidak N+1 query.
- Sediakan index dokumen reusable per `user_id + document_type + verification_status`.
- Simpan hasil review terakhir di requirement agar FE tidak perlu call terpisah untuk ambil AI message.
- Untuk initial page load, paling aman backend mendukung `include_default_detail=true` pada endpoint grouped.
- Dokumen cukup diakses dari `requirements[].document` pada `GET /dream-trackers/:id`, jadi FE tidak perlu endpoint dokumen terpisah.

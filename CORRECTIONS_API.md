# Corrections Management API

## Overview

Two new endpoints have been added to manage user corrections. These corrections are used to train the AI to better understand your spending patterns and classification preferences.

## Endpoints

### 1. List All Corrections
**GET** `/api/corrections`

Returns all corrections that have been saved. These are examples the AI uses to learn your preferences.

**Example Request:**
```bash
curl http://localhost:8080/api/corrections
```

**Example Response:**
```json
[
  {
    "id": 1,
    "raw_message": "starbucks 180",
    "category": "Food & Drink",
    "sub_category": "Coffee",
    "brand": "Starbucks",
    "behavior_tag": "treat",
    "created_at": "2025-04-26T10:30:00Z"
  },
  {
    "id": 2,
    "raw_message": "grab taxi 120",
    "category": "Transport",
    "sub_category": "Taxi",
    "brand": "Grab",
    "behavior_tag": "necessity",
    "created_at": "2025-04-25T15:20:00Z"
  }
]
```

### 2. Delete a Correction
**DELETE** `/api/corrections/:id`

Removes a correction by its ID. Use this when you want to remove a training example from the AI.

**Example Request:**
```bash
curl -X DELETE http://localhost:8080/api/corrections/1
```

**Example Response:**
```json
{
  "message": "correction deleted"
}
```

## Use Cases

### When to List Corrections
- Review what examples the AI is learning from
- Audit your correction history
- Check if there are conflicting corrections
- Export corrections for backup

### When to Delete Corrections
- Remove incorrect corrections
- Clean up duplicate examples
- Reset AI learning for specific patterns
- Remove outdated classification rules

## How Corrections Work

1. **Creating Corrections**: When you use `PATCH /api/transactions/:id/correct`, it:
   - Updates the transaction
   - Saves a correction example
   - AI uses this in future classifications

2. **AI Learning**: The AI includes recent corrections in its prompt as few-shot examples:
   ```
   Previous corrections you've learned:
   - "starbucks 180" → Food & Drink, Coffee, Starbucks, treat
   - "grab taxi 120" → Transport, Taxi, Grab, necessity
   ```

3. **Deleting Corrections**: When you delete a correction:
   - The transaction remains unchanged
   - Only the training example is removed
   - AI stops using this example in future classifications

## Integration with Existing Endpoints

### Existing Correction Endpoint
**PATCH** `/api/transactions/:id/correct`

This endpoint still works as before and automatically creates a correction entry.

**Example:**
```bash
curl -X PATCH http://localhost:8080/api/transactions/1/correct \
  -H "Content-Type: application/json" \
  -d '{
    "category": "Food & Drink",
    "sub_category": "Coffee",
    "brand": "Starbucks",
    "behavior_tag": "treat"
  }'
```

## Complete Workflow Example

```bash
# 1. Create a transaction via chat
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "starbucks 180"}'

# Response: { "id": 1, "brand": "Unknown", ... }

# 2. Correct the transaction
curl -X PATCH http://localhost:8080/api/transactions/1/correct \
  -H "Content-Type: application/json" \
  -d '{"brand": "Starbucks", "behavior_tag": "treat"}'

# 3. List all corrections to verify
curl http://localhost:8080/api/corrections

# 4. If you made a mistake, delete the correction
curl -X DELETE http://localhost:8080/api/corrections/1

# 5. The transaction still exists, but AI won't learn from it
curl http://localhost:8080/api/transactions/1
```

## API Summary

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/corrections` | List all corrections |
| DELETE | `/api/corrections/:id` | Delete a correction |
| PATCH | `/api/transactions/:id/correct` | Create/update correction (existing) |

## Testing

All endpoints are available in Swagger UI:
- Visit: `http://localhost:8080/swagger/index.html`
- Navigate to the "corrections" tag
- Try the endpoints interactively

## Notes

- Corrections are ordered by `created_at DESC` (newest first)
- Deleting a correction does NOT delete the transaction
- The AI uses the 100 most recent corrections for learning
- Corrections are stored in the `user_corrections` table

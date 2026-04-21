import express from 'express';

const app = express();

app.get('/api/users', async (req, res) => {
    const users = await db.query('SELECT * FROM users');
    res

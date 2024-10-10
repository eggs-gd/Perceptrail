import express from 'express';
import data from './photos.json' with { type: "json" };

const items = express.Router();

// middleware that is specific to this router
items.use(function timeLog(req, res, next) {
    console.log('Items', req.originalUrl, 'Time: ', Date.now());
    next();
});

// define the home page route
items.get('/', async function (req, res) {
    res.send(data);
});
items.post('/', function (req, res) {
    res.send('Add items');
})
items.put('/', function (req, res) {
    res.send('Edit items');
})

export default items;

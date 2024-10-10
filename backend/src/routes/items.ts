import express from 'express';

const items = express.Router();

// middleware that is specific to this router
items.use(function timeLog(req, res, next) {
    console.log('Time: ', Date.now());
    next();
});

// define the home page route
items.get('/', function (req, res) {
    res.send('Get items');
});
items.post('/', function(req, res){
    res.send('Add items');
})
items.put('/', function(req, res){
    res.send('Edit items');
})

export default items;

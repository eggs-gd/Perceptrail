import express from 'express';
import items from './routes/items.js'

const app = express();
const port = 3000;

app.use(function timeLog(req, res, next) {
    console.log('App', req.originalUrl, ' - Time: ', Date.now());
    next();
});

app.use('/items', items);

app.get('/', (req, res) => {
    res.send('The sedulous hyena ate the antelope!');
});

app.listen(port, () => {
    // if (err) {
    //     return console.error(err);
    // }
    return console.log(`server is listening on ${port}`);
});


